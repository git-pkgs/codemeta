package codemeta_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestValidation(t *testing.T) {
	input := `{"@context":"https://w3id.org/codemeta/3.0","author":[{"@type":"Organization","givenName":"A"}],"datePublished":"2023-02-29","license":"https://spdx.org/licenses/NotARegisteredLicense","typo":true,"type":12,"name":4}`
	doc, err := codemeta.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range doc.Validate() {
		got = append(got, d.Path+":"+d.Code)
		if d.Line != 1 || d.Column < 1 {
			t.Fatal(d)
		}
	}
	want := []string{"author[0]:agent_conflict", "datePublished:invalid_date", "typo:unknown_term", "type:value_type", "name:value_type"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
func TestDiagnosticOrderAndLimit(t *testing.T) {
	input := []byte("{\n\"name\":1,\n\"typo\":true,\n\"@context\":\"https://w3id.org/codemeta/3.0\",\n\"id\":4,\n\"@id\":false}")
	for _, limit := range []int{1, 2, 3, 4, 5} {
		doc, err := codemeta.ParseWithOptions(input, codemeta.ParseOptions{MaxDiagnostics: limit})
		if err != nil {
			t.Fatal(err)
		}
		ds := doc.Validate()
		if len(ds) != limit {
			t.Fatalf("limit %d: %+v", limit, ds)
		}
		if limit < 5 && ds[len(ds)-1].Code != "diagnostic_limit" {
			t.Fatal(ds)
		}
		for i, d := range ds {
			if i > 0 && ds[i-1].Line > d.Line {
				t.Fatal(ds)
			}
		}
	}
}
func TestValidationShapes(t *testing.T) {
	for _, body := range []string{
		`"@type":["SoftwareSourceCode","schema:CreativeWork"]`,
		`"author":{"@list":[{"givenName":"A"},{"@id":"https://example.org/a"}]}`,
		`"name":{"@value":"Nom","@language":"fr"}`,
		`"@graph":[{"name":"one"},{"name":"two"}]`,
		`"datePublished":["2024-02-29",null],"version":[2.0,"2.0"]`,
		`"startDate":"2024-02-29T12:00:00Z","position":1e2`,
	} {
		doc, err := codemeta.Parse([]byte(`{"@context":"https://w3id.org/codemeta/3.0",` + body + `}`))
		if err != nil {
			t.Fatal(err)
		}
		if ds := doc.Validate(); len(ds) != 0 {
			t.Fatalf("%s: %+v", body, ds)
		}
	}
}

func TestContainerKeywordValidation(t *testing.T) {
	for _, container := range []string{"@list", "@set"} {
		for _, tc := range []struct {
			field string
			want  []string
		}{
			{`"@index":"first"`, nil},
			{`"@index":5`, []string{"author.@index:value_type"}},
			{`"@type":5`, []string{"author.@type:container_field", "author.@type:value_type"}},
			{`"type":5`, []string{"author.type:container_field", "author.type:value_type"}},
			{`"id":5`, []string{"author.id:container_field", "author.id:value_type"}},
			{`"@language":5`, []string{"author.@language:container_field", "author.@language:value_type"}},
			{`"@unknown":true`, []string{"author.@unknown:container_field", "author.@unknown:unknown_keyword"}},
		} {
			input := `{"@context":"https://w3id.org/codemeta/3.0","author":{"` + container + `":[],` + tc.field + `}}`
			doc, err := codemeta.Parse([]byte(input))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, d := range doc.Validate() {
				got = append(got, d.Path+":"+d.Code)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("%s: got %v, want %v", input, got, tc.want)
			}
		}
	}
}

func TestLiteralValidation(t *testing.T) {
	for _, tc := range []struct{ literal, code, message string }{
		{`{"a":1}`, "value_type", "@value must be a scalar"},
		{`["one",{"a":1}]`, "value_type", "@value must be a scalar"},
		{`[]`, "value_type", "@value must be a scalar"},
		{`5`, "value_type", "expected Text"},
		{`"example"`, "", ""},
		{`null`, "", ""},
	} {
		input := `{"@context":"https://w3id.org/codemeta/3.0","name":{"@value":` + tc.literal + `,"@language":5}}`
		doc, err := codemeta.Parse([]byte(input))
		if err != nil {
			t.Fatal(err)
		}
		var want []codemeta.Diagnostic
		if tc.code != "" {
			want = append(want, codemeta.Diagnostic{Code: tc.code, Path: "name.@value", Message: tc.message,
				Position: codemeta.Position{Line: 1, Column: strings.Index(input, `"@value":`) + len(`"@value":`) + 1}})
		}
		want = append(want, codemeta.Diagnostic{Code: "value_type", Path: "name.@language", Message: "@language must be a string",
			Position: codemeta.Position{Line: 1, Column: strings.Index(input, `"@language":`) + len(`"@language":`) + 1}})
		if got := doc.Validate(); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %+v, want %+v", input, got, want)
		}
	}
}

func TestCalendarDatesAndIRIReferences(t *testing.T) {
	for _, tc := range []struct{ field, value, code string }{
		{"datePublished", `"2002"`, ""},
		{"datePublished", `"2024-02"`, ""},
		{"datePublished", `"2023-02-29"`, "invalid_date"},
		{"datePublished", `"2024-13"`, "invalid_date"},
		{"datePublished", `"2024-02-29"`, ""},
		{"referencePublication", `"https://example.org/paper"`, ""},
		{"referencePublication", `"paper.json"`, ""},
		{"referencePublication", `"not an IRI"`, "invalid_iri"},
		{"position", `1.00`, ""},
		{"position", `1e10000000000000000000`, ""},
		{"position", `0e-10000000000000000000`, ""},
		{"position", `1e-10000000000000000000`, "value_type"},
		{"position", `1.1`, "value_type"},
	} {
		doc, err := codemeta.Parse([]byte(`{"@context":"https://w3id.org/codemeta/3.0","` + tc.field + `":` + tc.value + `}`))
		if err != nil {
			t.Fatal(err)
		}
		ds := doc.Validate()
		if tc.code == "" {
			if len(ds) != 0 {
				t.Fatal(ds)
			}
		} else if len(ds) != 1 || ds[0].Code != tc.code {
			t.Fatalf("%s %s: %+v", tc.field, tc.value, ds)
		}
	}
}

func TestGraphPathsAndNestedUnknownFields(t *testing.T) {
	input := `{"@context":"https://w3id.org/codemeta/3.0","@graph":[{"name":"one","author":{"givenName":"Ada","unknown":{"inner":true}}},{"name":"two","datePublished":"2023-02-29"}]}`
	doc, err := codemeta.Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	graph := doc.Get("@graph").Items()
	if len(graph) != 2 || graph[0].Get("name").Text() != "one" || graph[1].Get("name").Text() != "two" {
		t.Fatal("graph content changed")
	}
	if graph[0].Get("author").Get("unknown").Get("inner").Kind() != codemeta.Boolean {
		t.Fatal("unknown nested field lost")
	}
	var paths []string
	for _, d := range doc.Validate() {
		paths = append(paths, d.Path+":"+d.Code)
	}
	want := []string{"@graph[0].author.unknown:unknown_term", "@graph[0].author.unknown.inner:unknown_term", "@graph[1].datePublished:invalid_date"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("%v, want %v", paths, want)
	}
}
