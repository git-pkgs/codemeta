package codemeta

import (
	"net/url"
	"strings"
	"time"
	"unicode"
)

func isAgentRange(expected string) bool {
	for _, name := range strings.Fields(expected) {
		if name == rangePerson || name == rangeOrganization {
			return true
		}
	}
	return false
}
func matchesExpected(v Value, expected string) bool {
	for _, typ := range strings.Fields(expected) {
		switch typ {
		case rangeText, rangeURL, rangeDate, rangeDatetime:
			if v.kind == String {
				return true
			}
		case rangeNumber:
			if v.kind == Number {
				return true
			}
		case rangeInteger:
			if v.kind == Number && integerNumber(v.text) {
				return true
			}
		case rangeBoolean:
			if v.kind == Boolean {
				return true
			}
		default:
			if v.kind == Object {
				return true
			}
		}
	}
	return false
}
func integerNumber(text string) bool {
	// JSON integers written with a decimal or exponent are evaluated without float conversion.
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(text), "e")
	scale := 0
	if hasExponent {
		sign := 1
		if strings.HasPrefix(exponent, "-") {
			sign = -1
		}
		exponent = strings.TrimLeft(exponent, "+-0")
		for _, r := range exponent {
			if scale > len(text) {
				break
			}
			scale = scale*decimalBase + int(r-'0')
		}
		scale *= sign
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(strings.TrimPrefix(whole, "-")+fraction, "0")
	if digits == "" {
		return true
	}
	required := len(fraction) - scale
	if required <= 0 {
		return true
	}
	if required > len(digits) {
		return false
	}
	return strings.Trim(digits[len(digits)-required:], "0") == ""
}
func validURL(text string) bool {
	if text == "" || strings.ContainsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	u, err := url.Parse(text)
	if err != nil || u.Scheme == "" {
		return false
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return u.Host != ""
	}
	return u.Opaque != "" || u.Host != "" || u.Path != ""
}
func validDate(text string, datetime bool) bool {
	for _, layout := range []string{"2006", "2006-01", "2006-01-02"} {
		if _, err := time.Parse(layout, text); err == nil {
			return true
		}
	}
	if datetime {
		_, err := time.Parse(time.RFC3339, text)
		return err == nil
	}
	return false
}
func (c *validator) stringValue(v Value, def termDefinition, path string) {
	types := strings.Fields(def.expected)
	text, date, datetime, urlType := false, false, false, false
	for _, typ := range types {
		switch typ {
		case rangeText:
			text = true
		case rangeDate:
			date = true
		case rangeDatetime:
			datetime = true
		case rangeURL:
			urlType = true
		}
	}
	if normalizeIRI(def.coercion) == schemaDate {
		date = true
	}
	if !text && (date || datetime) && !validDate(v.text, datetime) {
		c.add(v.pos, path, "invalid_date", "invalid calendar date or date-time")
	}
	if !text && urlType && !validURL(v.text) {
		c.add(v.pos, path, "invalid_url", "expected an absolute URL")
	}
	if !text && !urlType && def.coercion == keywordID && !validIRI(v.text) {
		c.add(v.pos, path, "invalid_iri", "expected an IRI reference")
	}
}

func validIRI(text string) bool {
	if strings.ContainsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) {
		return false
	}
	_, err := url.Parse(text)
	return err == nil
}

const schemaDate = "https://schema.org/Date"
