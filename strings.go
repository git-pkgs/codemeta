package codemeta

// Strings projects a term's scalar spelling, object identifiers, or object names
// in source order. It unwraps JSON-LD lists, sets, and value objects and resolves
// keys using each object's context. Empty strings and nulls are skipped. The
// original values remain available through Get.
func (d *Document) Strings(term string) []string {
	if d == nil {
		return nil
	}
	var terms map[string]termDefinition
	if d.context.usable() {
		terms = contextTerms[d.context.version]
	}
	return stringValues(d.Get(term), terms)
}

// Strings projects an agent field using the agent's local context.
func (a Agent) Strings(term string) []string {
	return stringValues(a.Get(term), a.terms)
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func stringValues(value Value, terms map[string]termDefinition) []string {
	return appendStringValues(nil, value, terms)
}

func appendStringValues(values []string, value Value, terms map[string]termDefinition) []string {
	for _, item := range value.Values() {
		if item.kind == String || item.kind == Number || item.kind == Boolean {
			if item.text != "" {
				values = append(values, item.text)
			}
			continue
		}
		if item.kind != Object {
			continue
		}
		localTerms := scopedTerms(item, terms)
		// An empty identifier means absent, so the search falls through to a name;
		// an empty list, set, or value is genuinely empty and stops the search.
		for _, key := range []string{keywordList, keywordSet, keywordValue, keywordID, nameTerm} {
			field := resolvedGet(item, key, localTerms)
			if field.kind == Missing {
				continue
			}
			count := len(values)
			values = appendStringValues(values, field, localTerms)
			if key != keywordID || len(values) != count {
				break
			}
		}
	}
	return values
}
