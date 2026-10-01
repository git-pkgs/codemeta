package codemeta_test

import (
	"reflect"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestAdditionalPropertyRanges(t *testing.T) {
	for _, tc := range []struct {
		version, term string
		values        []any
	}{
		{"2.0", "contIntegration", []any{"https://example.org/ci"}},
		{"2.0", "creator", []any{map[string]any{"@type": "Person", "name": "Ada"}}},
		{"2.0", "embargoDate", []any{"2024-02-29"}},
		{"master", "archivedAt", []any{"https://example.org/archive", map[string]any{"name": "Archive"}}},
		{"master", "encodingFormat", []any{"application/json"}},
		{"master", "featureList", []any{"Offline parsing"}},
	} {
		t.Run(tc.term, func(t *testing.T) {
			for _, value := range tc.values {
				for _, shape := range []any{value, []any{value}, map[string]any{"@set": []any{value}}} {
					doc := parse(t, map[string]any{"@context": contextURL(tc.version), tc.term: shape})
					if ds := doc.Validate(); len(ds) != 0 {
						t.Fatalf("valid value: %+v", ds)
					}
				}
			}
			doc := parse(t, map[string]any{"@context": contextURL(tc.version), tc.term: true})
			if ds := doc.Validate(); len(ds) != 1 || ds[0].Code != "value_type" {
				t.Fatalf("invalid value: %+v", ds)
			}
		})
	}
}

func TestIdentifierReferenceForms(t *testing.T) {
	for _, id := range []string{"", "#person", "../person", "urn:example:person", "_:person", "https://example.org/person"} {
		for _, key := range []string{"@id", "id"} {
			doc := parse(t, map[string]any{"@context": v3, "author": map[string]any{key: id}})
			if ds := doc.Validate(); len(ds) != 0 {
				t.Fatalf("%s=%q: %+v", key, id, ds)
			}
			if a := doc.Author()[0]; a.Kind() != codemeta.AgentReference || a.Identifier().Text() != id {
				t.Fatalf("reference changed: %+v", a)
			}
		}
	}
}

func TestAgentTextAndEmptyContainers(t *testing.T) {
	doc := parse(t, map[string]any{"@context": v3, "author": []any{nil, map[string]any{"@set": nil}, "Ada"}})
	if authors := doc.Author(); len(authors) != 1 || authors[0].Name() != "Ada" || authors[0].Kind() != codemeta.AgentText {
		t.Fatalf("authors: %+v", authors)
	}
}

func TestAgentStringsLocalContext(t *testing.T) {
	for _, tc := range []struct {
		context any
		want    []string
	}{
		{map[string]any{}, []string{"Ada", "Grace"}},
		{[]any{map[string]any{}}, []string{"Ada", "Grace"}},
		{[]any{}, []string{"Ada", "Grace"}},
		{nil, nil},
		{map[string]any{"schema": "https://example.org/"}, nil},
	} {
		doc := parse(t, map[string]any{"@context": v3, "author": map[string]any{
			"@context": tc.context, "schema:name": map[string]any{"@set": []any{"Ada", "Grace"}},
		}})
		if got := doc.Author()[0].Strings("name"); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("context %v: %v, want %v", tc.context, got, tc.want)
		}
	}
}

func TestLiteralDirection(t *testing.T) {
	for _, direction := range []any{nil, "ltr", "rtl"} {
		doc := parse(t, map[string]any{"@context": v3, "name": map[string]any{"@value": "Example", "@direction": direction}})
		if ds := doc.Validate(); len(ds) != 0 {
			t.Fatal(ds)
		}
	}
	doc := parse(t, map[string]any{"@context": v3, "name": map[string]any{"@value": "Example", "@direction": "sideways"}})
	if ds := doc.Validate(); len(ds) != 1 || ds[0].Code != "invalid_direction" {
		t.Fatal(ds)
	}
}

func FuzzMetadataProjection(f *testing.F) {
	for _, seed := range []string{
		`{"@context":"https://w3id.org/codemeta/3.0","name":{"@value":"Example"},"author":[{"@set":[{"givenName":"Ada"}]}]}`,
		`{"@context":"https://w3id.org/codemeta/3.0","author":{"@context":[{}],"schema:name":{"@list":["Ada","Grace"]}}}`,
		`{"author":{"@type":"Role","roleName":{"@value":"creator"},"author":{"name":"Ada"}}}`,
		`{"name":[null,{},[],false,12],"author":[null,{"name":null},{"@id":"_:person"}]}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := codemeta.Parse(data)
		if err != nil {
			return
		}
		for _, field := range doc.Fields() {
			if !reflect.DeepEqual(doc.Strings(field.Name), doc.Strings(field.Name)) {
				t.Fatal("nondeterministic document projection")
			}
		}
		for _, relation := range []string{"author", "contributor", "maintainer", "copyrightHolder", "funder"} {
			checkAgentProjection(t, agents(doc, relation))
		}
	})
}

func checkAgentProjection(t *testing.T, values []codemeta.Agent) {
	t.Helper()
	for _, agent := range values {
		for _, field := range agent.Value().Fields() {
			if !reflect.DeepEqual(agent.Strings(field.Name), agent.Strings(field.Name)) {
				t.Fatal("nondeterministic agent projection")
			}
		}
		checkAgentProjection(t, agent.Agents())
	}
}
