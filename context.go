package codemeta

import "strings"

//go:generate go run ./internal/generate

// Version identifies a pinned context, independently of software metadata.
type Version string

const (
	UnknownVersion Version = ""
	Version2       Version = "2.0"
	Version3       Version = "3.0"
	VersionMaster  Version = "master"
)

type termDefinition struct {
	iri, coercion, container, expected string
	prefix                             bool
}
type contextState struct {
	version                                 Version
	missing, unknown, modified, conflicting bool
}

func (c contextState) usable() bool {
	return c.version != "" && !c.unknown && !c.modified && !c.conflicting
}
func (d *Document) Version() Version {
	if d == nil {
		return UnknownVersion
	}
	return d.context.version
}
func inspectContext(v Value) contextState {
	c := contextState{missing: v.kind == Missing}
	c.include(v)
	if c.conflicting {
		c.version = UnknownVersion
	}
	return c
}
func (c *contextState) include(v Value) {
	switch v.kind {
	case Missing:
		return
	case Array:
		if len(v.items) == 0 {
			c.unknown = true
		}
		for _, item := range v.items {
			if item.kind == Array {
				c.unknown = true
			} else {
				c.include(item)
			}
		}
	case String:
		c.selectVersion(contextURL(v.text))
	case Object:
		if len(v.fields) == 0 || (c.version != UnknownVersion && unchangedDefinitions(v, contextTerms[c.version])) {
			return
		}
		for _, version := range []Version{Version2, Version3, VersionMaster} {
			if sameContext(v, contextTerms[version]) {
				c.selectVersion(version)
				return
			}
		}
		c.modified = true
	default:
		c.unknown = true
	}
}
func (c *contextState) selectVersion(v Version) {
	if v == UnknownVersion {
		c.unknown = true
		return
	}
	if c.version != "" && c.version != v {
		c.conflicting = true
	}
	c.version = v
}
func contextURL(text string) Version {
	if !strings.HasPrefix(text, "https://") && !strings.HasPrefix(text, "http://") {
		return UnknownVersion
	}
	text = strings.TrimSuffix(text, "/")
	text = strings.TrimPrefix(strings.TrimPrefix(text, "https://"), "http://")
	switch text {
	case "w3id.org/codemeta/2.0", "doi.org/10.5063/schema/codemeta-2.0",
		"raw.githubusercontent.com/codemeta/codemeta/2.0/codemeta.jsonld",
		"raw.githubusercontent.com/codemeta/codemeta/dbc58ffa8b088ae1b09ad335852d9767c402983d/codemeta.jsonld":
		return Version2
	case "w3id.org/codemeta/3.0", "raw.githubusercontent.com/codemeta/codemeta/3.0/codemeta.jsonld",
		"raw.githubusercontent.com/codemeta/codemeta/19a4de2deb55ab7c907984bc9333f45eb5f50412/codemeta.jsonld":
		return Version3
	case "raw.githubusercontent.com/codemeta/codemeta/master/codemeta.jsonld",
		"raw.githubusercontent.com/codemeta/codemeta/3657b0fa6c75cbf8e541ac1ab9af9acfb080f2c0/codemeta.jsonld":
		return VersionMaster
	default:
		return UnknownVersion
	}
}
func sameContext(v Value, terms map[string]termDefinition) bool {
	if len(v.fields) != len(terms) {
		return false
	}
	return unchangedDefinitions(v, terms)
}

func unchangedDefinitions(v Value, terms map[string]termDefinition) bool {
	for _, f := range v.fields {
		def, ok := terms[f.Name]
		if !ok {
			return false
		}
		value := f.Value
		switch value.kind {
		case String:
			if expandIRI(value.text, terms) != normalizeIRI(def.iri) || def.container != "" || def.coercion != "" {
				return false
			}
		case Object:
			for _, key := range value.fields {
				if key.Value.kind != String || (key.Name != keywordID && key.Name != keywordType && key.Name != keywordContainer) {
					return false
				}
			}
			if expandIRI(value.Get(keywordID).text, terms) != normalizeIRI(def.iri) || expandIRI(value.Get(keywordType).text, terms) != normalizeIRI(def.coercion) || value.Get(keywordContainer).text != def.container {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func scopedTerms(v Value, inherited map[string]termDefinition) map[string]termDefinition {
	if local := v.Get(keywordContext); local.kind != Missing {
		if local.kind == Object && unchangedDefinitions(local, inherited) {
			return inherited
		}
		state := inspectContext(local)
		if !state.usable() {
			return nil
		}
		return contextTerms[state.version]
	}
	return inherited
}
func normalizeIRI(iri string) string {
	if rest, ok := strings.CutPrefix(iri, "http://schema.org/"); ok {
		return "https://schema.org/" + rest
	}
	return iri
}
func expandIRI(name string, terms map[string]termDefinition) string {
	if def, ok := terms[name]; ok {
		return normalizeIRI(def.iri)
	}
	if prefix, local, ok := strings.Cut(name, ":"); ok {
		if def, exists := terms[prefix]; exists && def.prefix {
			return normalizeIRI(def.iri + local)
		}
	}
	return normalizeIRI(name)
}
func resolveTerm(name string, terms map[string]termDefinition) (termDefinition, bool) {
	if def, ok := terms[name]; ok {
		return def, true
	}
	iri := expandIRI(name, terms)
	for _, def := range terms {
		if normalizeIRI(def.iri) == iri {
			return def, true
		}
	}
	return termDefinition{}, false
}
func resolvedGet(v Value, name string, terms map[string]termDefinition) Value {
	if exact := v.Get(name); exact.kind != Missing {
		return exact
	}
	iri := expandIRI(name, terms)
	for _, field := range v.fields {
		if expandIRI(field.Name, terms) == iri {
			return field.Value
		}
	}
	return Value{}
}

const (
	keywordContext    = "@context"
	keywordID         = "@id"
	keywordType       = "@type"
	keywordList       = "@list"
	keywordSet        = "@set"
	keywordValue      = "@value"
	keywordContainer  = "@container"
	keywordIndex      = "@index"
	rangeText         = "Text"
	rangeURL          = "URL"
	rangeDate         = "Date"
	rangeDatetime     = "Datetime"
	rangeNumber       = "Number"
	rangeInteger      = "Integer"
	rangeBoolean      = "Boolean"
	rangePerson       = "Person"
	rangeOrganization = "Organization"
)
