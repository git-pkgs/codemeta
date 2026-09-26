package codemeta

import (
	"fmt"
	"sort"
	"strings"
)

type validator struct {
	diagnostics []Diagnostic
	limit       int
	truncated   bool
}

// Validate returns diagnostics in source order, capped including a truncation entry.
func (d *Document) Validate() []Diagnostic {
	if d == nil {
		return nil
	}
	limit := d.maxDiagnostics
	if limit == 0 {
		limit = 100
	}
	c := validator{limit: limit}
	c.context(d.root.Get(keywordContext), keywordContext, d.root.pos)
	var terms map[string]termDefinition
	if d.context.usable() {
		terms = contextTerms[d.context.version]
	}
	c.object(d.root, "", terms)
	if c.truncated {
		last := c.diagnostics[len(c.diagnostics)-1]
		c.diagnostics[len(c.diagnostics)-1] = Diagnostic{Code: "diagnostic_limit", Path: last.Path, Message: "further diagnostics omitted", Position: last.Position}
	}
	return c.diagnostics
}
func diagnosticLess(a, b Diagnostic) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	if a.Column != b.Column {
		return a.Column < b.Column
	}
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Code < b.Code
}
func (c *validator) add(pos Position, path, code, message string) {
	d := Diagnostic{Position: pos, Path: path, Code: code, Message: message}
	i := sort.Search(len(c.diagnostics), func(i int) bool { return !diagnosticLess(c.diagnostics[i], d) })
	if len(c.diagnostics) < c.limit {
		c.diagnostics = append(c.diagnostics, Diagnostic{})
	} else {
		c.truncated = true
		if i == len(c.diagnostics) {
			return
		}
	}
	copy(c.diagnostics[i+1:], c.diagnostics[i:len(c.diagnostics)-1])
	c.diagnostics[i] = d
}
func (c *validator) context(v Value, path string, fallback Position) {
	state := inspectContext(v)
	pos := v.pos
	if pos.Line == 0 {
		pos = fallback
	}
	if state.missing {
		c.add(pos, path, "missing_context", "@context is missing")
	}
	if state.unknown || (state.version == "" && !state.missing && !state.modified && !state.conflicting) {
		c.add(pos, path, "unsupported_version", "context is not one of the pinned CodeMeta contexts")
	}
	if state.modified {
		c.add(pos, path, "context_modified", "inline context definitions cannot be applied using the pinned tables")
	}
	if state.conflicting {
		c.add(pos, path, "context_conflict", "context combines different CodeMeta versions")
	}
}
func fieldPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}
func itemPath(parent string, i int) string { return fmt.Sprintf("%s[%d]", parent, i) }
func (c *validator) object(v Value, path string, terms map[string]termDefinition) {
	if path != "" {
		if local := v.Get(keywordContext); local.kind != Missing {
			if local.kind != Object || !unchangedDefinitions(local, terms) {
				c.context(local, fieldPath(path, keywordContext), v.pos)
			}
			terms = scopedTerms(v, terms)
		}
	}
	seen := map[string]bool{}
	for _, field := range v.fields {
		if field.Name == keywordContext {
			continue
		}
		childPath := fieldPath(path, field.Name)
		name := expandIRI(field.Name, terms)
		if seen[name] {
			c.add(field.Position, childPath, "alias_conflict", "multiple keys resolve to the same term")
		}
		seen[name] = true
		if strings.HasPrefix(name, "@") {
			c.keyword(field.Value, name, childPath, terms)
			continue
		}
		def, known := resolveTerm(field.Name, terms)
		if terms != nil && !known {
			code := "unknown_term"
			if knownElsewhere(field.Name) {
				code = "term_version"
			}
			c.add(field.Position, childPath, code, "term is not defined by the declared context")
		}
		c.property(field.Value, def, childPath, terms)
	}
}
func knownElsewhere(name string) bool {
	for _, terms := range contextTerms {
		if _, ok := resolveTerm(name, terms); ok {
			return true
		}
	}
	return false
}
func (c *validator) property(v Value, def termDefinition, path string, terms map[string]termDefinition) {
	if v.kind == Array {
		for i, item := range v.items {
			c.property(item, def, itemPath(path, i), terms)
		}
		return
	}
	if v.kind == Null {
		return
	}
	if v.kind == Object {
		if list := v.Get(keywordList); list.kind != Missing {
			c.property(list, def, fieldPath(path, keywordList), terms)
			c.container(v, keywordList, path, terms)
			return
		}
		if set := v.Get(keywordSet); set.kind != Missing {
			c.property(set, def, fieldPath(path, keywordSet), terms)
			c.container(v, keywordSet, path, terms)
			return
		}
		if literal := v.Get(keywordValue); literal.kind != Missing {
			if literal.kind != Object && literal.kind != Array {
				c.property(literal, def, fieldPath(path, keywordValue), terms)
			}
			c.object(v, path, terms)
			return
		}
	}
	reference := v.kind == String && def.coercion == keywordID
	referenceObject := v.kind == Object && resolvedGet(v, keywordID, scopedTerms(v, terms)).kind == String
	if def.expected != "" && !reference && !referenceObject && !matchesExpected(v, def.expected) {
		c.add(v.pos, path, "value_type", "expected "+def.expected)
	}
	if v.kind == String {
		c.stringValue(v, def, path)
	}
	if v.kind == Object {
		if isAgentRange(def.expected) {
			agent := Agent{value: v, terms: scopedTerms(v, terms)}
			if agent.Kind() == AgentConflict {
				c.add(v.pos, path, "agent_conflict", "agent has conflicting person and organization fields")
			}
		}
		c.object(v, path, terms)
	}
}
func (c *validator) container(v Value, key, path string, terms map[string]termDefinition) {
	for _, field := range v.fields {
		if field.Name == key {
			continue
		}
		name := expandIRI(field.Name, terms)
		childPath := fieldPath(path, field.Name)
		if name != keywordIndex {
			c.add(field.Position, childPath, "container_field", "list and set objects may only also contain @index")
		}
		if strings.HasPrefix(name, "@") {
			c.keyword(field.Value, name, childPath, terms)
		} else {
			c.property(field.Value, termDefinition{}, childPath, terms)
		}
	}
}
func (c *validator) keyword(v Value, name, path string, terms map[string]termDefinition) {
	switch name {
	case keywordID:
		if v.kind != String {
			c.add(v.pos, path, "value_type", "@id must be a string")
		}
	case keywordType:
		c.typeValue(v, path, terms)
	case "@graph", "@included", "@reverse":
		c.property(v, termDefinition{}, path, terms)
	case keywordList, keywordSet:
		c.property(v, termDefinition{}, path, terms)
	case keywordValue:
		if v.kind == Object || v.kind == Array {
			c.add(v.pos, path, "value_type", "@value must be a scalar")
		}
	case "@language", "@direction", keywordIndex:
		if v.kind != String {
			c.add(v.pos, path, "value_type", name+" must be a string")
		}
	default:
		c.add(v.pos, path, "unknown_keyword", "unknown JSON-LD keyword")
	}
}
func (c *validator) typeValue(v Value, path string, terms map[string]termDefinition) {
	if v.kind == Array {
		for i, item := range v.items {
			if item.kind != String {
				c.add(item.pos, itemPath(path, i), "value_type", "@type items must be strings")
			} else {
				c.typeValue(item, itemPath(path, i), terms)
			}
		}
		return
	}
	if v.kind != String {
		c.add(v.pos, path, "value_type", "@type must be a string or an array of strings")
		return
	}
	if terms == nil {
		return
	}
	if _, ok := resolveTerm(v.text, terms); ok {
		return
	}
	if iri := expandIRI(v.text, terms); strings.Contains(iri, ":") {
		return
	}
	code := "unknown_type"
	if knownElsewhere(v.text) {
		code = "type_version"
	}
	c.add(v.pos, path, code, "type is not defined by the declared context")
}
