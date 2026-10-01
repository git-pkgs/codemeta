package codemeta_test

import (
	"reflect"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestDocumentStrings(t *testing.T) {
	for _, tc := range []struct {
		name, input, term string
		want              []string
	}{
		{"scalar", `{"keywords":"science"}`, "keywords", []string{"science"}},
		{"array", `{"keywords":["science","metadata"]}`, "keywords", []string{"science", "metadata"}},
		{"set", `{"keywords":{"@set":["science","metadata"]}}`, "keywords", []string{"science", "metadata"}},
		{"list", `{"keywords":{"@list":["science",{"@value":"metadata"}]}}`, "keywords", []string{"science", "metadata"}},
		{"id alias", `{"@context":"https://w3id.org/codemeta/3.0","license":{"id":"https://spdx.org/licenses/MIT"}}`, "license", []string{"https://spdx.org/licenses/MIT"}},
		{"prefixed names", `{"@context":"https://w3id.org/codemeta/3.0","schema:programmingLanguage":[{"schema:name":"Go"},{"http://schema.org/name":"Ruby"}]}`, "programmingLanguage", []string{"Go", "Ruby"}},
		{"identifier first", `{"license":{"@id":"MIT","name":"MIT License"}}`, "license", []string{"MIT"}},
		{"empty identifier", `{"license":{"@id":"","name":"MIT License"}}`, "license", []string{"MIT License"}},
		{"exact key first", `{"@context":"https://w3id.org/codemeta/3.0","programmingLanguage":{"name":"Go","schema:name":"Ruby"}}`, "programmingLanguage", []string{"Go"}},
		{"local context", `{"programmingLanguage":{"@context":"https://w3id.org/codemeta/3.0","schema:name":"Go"}}`, "programmingLanguage", []string{"Go"}},
		{"reset context", `{"@context":"https://w3id.org/codemeta/3.0","programmingLanguage":{"@context":null,"schema:name":"Go"}}`, "programmingLanguage", nil},
		{"unsupported context", `{"@context":"https://example.org/context","programmingLanguage":{"schema:name":"Go"}}`, "programmingLanguage", nil},
		{"scalars", `{"keywords":[null,4,true,"",{},"science"]}`, "keywords", []string{"4", "true", "science"}},
		{"number spelling", `{"copyrightYear":2020}`, "copyrightYear", []string{"2020"}},
		{"number value object", `{"copyrightYear":{"@value":2020}}`, "copyrightYear", []string{"2020"}},
		{"missing", `{}`, "keywords", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := codemeta.Parse([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			before := doc.Get(tc.term)
			if got := doc.Strings(tc.term); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Strings(%q) = %v, want %v", tc.term, got, tc.want)
			}
			if !reflect.DeepEqual(doc.Get(tc.term), before) {
				t.Fatal("projection changed the source value")
			}
		})
	}
}
