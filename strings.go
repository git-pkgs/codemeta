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

func stringValues(value Value, terms map[string]termDefinition) []string {
	var values []string
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
			projected := stringValues(field, localTerms)
			values = append(values, projected...)
			if key != keywordID || len(projected) != 0 {
				break
			}
		}
	}
	return values
}
