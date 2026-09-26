package codemeta_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestContexts(t *testing.T) {
	for _, version := range []codemeta.Version{codemeta.Version2, codemeta.Version3, codemeta.VersionMaster} {
		urls := []string{"https://w3id.org/codemeta/" + string(version), "https://raw.githubusercontent.com/codemeta/codemeta/" + string(version) + "/codemeta.jsonld"}
		if version == codemeta.VersionMaster {
			urls = urls[1:]
		}
		if version == codemeta.Version2 {
			urls = append(urls, "https://doi.org/10.5063/schema/codemeta-2.0")
		}
		for _, url := range urls {
			checkContextURLForms(t, url, version)
		}
		data, err := os.ReadFile("internal/contexts/" + string(version) + ".jsonld")
		if err != nil {
			t.Fatal(err)
		}
		var pinned map[string]json.RawMessage
		if err := json.Unmarshal(data, &pinned); err != nil {
			t.Fatal(err)
		}
		doc, err := codemeta.Parse([]byte(`{"@context":` + string(pinned["@context"]) + `}`))
		if err != nil || doc.Version() != version || len(doc.Validate()) != 0 {
			t.Fatalf("inline %s: %v %v", version, doc, err)
		}
	}
}

func checkContextURLForms(t *testing.T, url string, version codemeta.Version) {
	t.Helper()
	for _, scheme := range []string{"http://", "https://"} {
		for _, slash := range []string{"", "/"} {
			context := scheme + strings.TrimPrefix(url, "https://") + slash
			for _, array := range []bool{false, true} {
				value := `"` + context + `"`
				if array {
					value = "[" + value + "]"
				}
				doc, err := codemeta.Parse([]byte(`{"@context":` + value + `}`))
				if err != nil || doc.Version() != version || len(doc.Validate()) != 0 {
					t.Fatalf("%s: %v %v", value, doc, err)
				}
			}
		}
	}
}
func TestUnusableContexts(t *testing.T) {
	cases := []struct{ input, code string }{
		{`{}`, "missing_context"},
		{`{"@context":"https://w3id.org/codemeta/4.0"}`, "unsupported_version"},
		{`{"@context":"https://w3id.org/codemeta/3.1"}`, "unsupported_version"},
		{`{"@context":"https://w3id.org/codemeta/2.1"}`, "unsupported_version"},
		{`{"@context":[]}`, "unsupported_version"},
		{`{"@context":[["https://w3id.org/codemeta/3.0"]]}`, "unsupported_version"},
		{`{"@context":["https://w3id.org/codemeta/3.0",[]],"name":42}`, "unsupported_version"},
		{`{"@context":{"name":"https://example.org/name"}}`, "context_modified"},
		{`{"@context":["https://w3id.org/codemeta/3.0",{"name":"https://example.org/name"}],"name":42}`, "context_modified"},
		{`{"@context":["https://w3id.org/codemeta/2.0","https://w3id.org/codemeta/3.0"]}`, "context_conflict"},
	}
	for _, tc := range cases {
		doc, err := codemeta.Parse([]byte(tc.input))
		if err != nil {
			t.Fatal(err)
		}
		ds := doc.Validate()
		if len(ds) != 1 || ds[0].Code != tc.code {
			t.Fatalf("%s: %+v", tc.input, ds)
		}
	}
}
func TestContextArrayOrder(t *testing.T) {
	for _, tc := range []struct {
		context, name string
		codes         []string
	}{
		{`["https://w3id.org/codemeta/3.0",{"name":"schema:name"}]`, "demo", nil},
		{`[{"name":"schema:name"},"https://w3id.org/codemeta/3.0"]`, "", []string{"context_modified"}},
	} {
		doc, err := codemeta.Parse([]byte(`{"@context":` + tc.context + `,"schema:name":"demo"}`))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Version() != codemeta.Version3 || doc.Name() != tc.name || doc.Get("schema:name").Text() != "demo" {
			t.Fatalf("context %s: version=%s name=%q", tc.context, doc.Version(), doc.Name())
		}
		var codes []string
		for _, d := range doc.Validate() {
			codes = append(codes, d.Code)
		}
		if !reflect.DeepEqual(codes, tc.codes) {
			t.Fatalf("context %s: diagnostics=%+v", tc.context, doc.Validate())
		}
	}
}

func TestVersionTermChanges(t *testing.T) {
	for _, tc := range []struct{ version, term, code string }{
		{"2.0", "continuousIntegration", "term_version"}, {"2.0", "contIntegration", ""},
		{"3.0", "continuousIntegration", ""}, {"3.0", "contIntegration", "term_version"},
		{"3.0", "contactPoint", "term_version"}, {"master", "contactPoint", ""},
		{"2.0", "typo", "unknown_term"},
	} {
		url := "https://w3id.org/codemeta/" + tc.version
		if tc.version == "master" {
			url = "https://raw.githubusercontent.com/codemeta/codemeta/master/codemeta.jsonld"
		}
		input := `{"@context":"` + url + `","` + tc.term + `":null}`
		doc, err := codemeta.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		ds := doc.Validate()
		if tc.code == "" {
			if len(ds) != 0 {
				t.Fatal(ds)
			}
		} else if len(ds) != 1 || ds[0].Code != tc.code {
			t.Fatalf("%s: %+v", input, ds)
		}
	}
}

func TestContextDefinitionsAndNestedScopes(t *testing.T) {
	cases := []struct {
		body  string
		codes []string
	}{
		{`"@context":["https://w3id.org/codemeta/3.0",{}],"name":"example"`, nil},
		{`"@context":["https://w3id.org/codemeta/3.0",{"name":"schema:name"}],"name":"example"`, nil},
		{`"@context":["https://w3id.org/codemeta/3.0",{"name":{"@id":"schema:name","@type":null}}],"name":3`, []string{"context_modified"}},
		{`"@context":"https://w3id.org/codemeta/3.0","author":{"@context":{},"schema:givenName":"Ada"}`, nil},
		{`"@context":"https://w3id.org/codemeta/3.0","author":{"@context":[["https://w3id.org/codemeta/3.0"]],"schema:givenName":"Ada"}`, []string{"unsupported_version"}},
		{`"@context":"https://w3id.org/codemeta/3.0","author":{"@context":{"givenName":"schema:givenName"},"schema:givenName":"Ada"}`, nil},
		{`"@context":"https://w3id.org/codemeta/3.0","author":{"@context":{"schema":"https://example.org/"},"schema:givenName":"Ada"}`, []string{"context_modified"}},
	}
	for _, tc := range cases {
		doc, err := codemeta.Parse([]byte(`{` + tc.body + `}`))
		if err != nil {
			t.Fatal(err)
		}
		ds := doc.Validate()
		var codes []string
		for _, d := range ds {
			codes = append(codes, d.Code)
		}
		if !reflect.DeepEqual(codes, tc.codes) {
			t.Fatalf("%s: %+v", tc.body, ds)
		}
		if len(doc.Author()) != 0 && len(tc.codes) == 0 && doc.Author()[0].GivenName() != "Ada" {
			t.Fatal("nested context lost")
		}
		if len(doc.Author()) != 0 && len(tc.codes) > 0 && doc.Author()[0].GivenName() != "" {
			t.Fatal("modified prefix was resolved using the pinned table")
		}
	}
}
