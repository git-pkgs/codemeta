package codemeta_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	cm "github.com/git-pkgs/codemeta"
)

const v3 = "https://w3id.org/codemeta/3.0"

func parse(t *testing.T, input any) *cm.Document {
	t.Helper()
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	d, err := cm.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
func contextURL(v string) string {
	if v == "master" {
		return "https://raw.githubusercontent.com/codemeta/codemeta/master/codemeta.jsonld"
	}
	return "https://w3id.org/codemeta/" + v
}
func contexts(t *testing.T, v string) map[string]any {
	t.Helper()
	b, e := os.ReadFile("internal/contexts/" + v + ".jsonld")
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Context map[string]any `json:"@context"`
	}
	if e = json.Unmarshal(b, &doc); e != nil {
		t.Fatal(e)
	}
	return doc.Context
}
func iri(ctx map[string]any, term string) string {
	d := ctx[term]
	s, ok := d.(string)
	if !ok {
		s, _ = d.(map[string]any)["@id"].(string)
	}
	p, rest, ok := strings.Cut(s, ":")
	if ok && p != "http" && p != "https" {
		if base, ok := ctx[p].(string); ok {
			s = base + rest
		}
	}
	return s
}
func properties(ctx map[string]any) []string {
	var out []string
	for term := range ctx {
		if term == "id" || term == "type" || term == "schema" || term == "codemeta" || term[0] < 'a' || term[0] > 'z' {
			continue
		}
		out = append(out, term)
	}
	sort.Strings(out)
	return out
}
func TestEveryPropertyProjection(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  []string
	}{
		{"text", "alpha", []string{"alpha"}}, {"number", json.Number("1.20"), []string{"1.20"}}, {"boolean", false, []string{"false"}},
		{"empty", "", nil}, {"null", nil, nil}, {"array", []any{"alpha", "beta"}, []string{"alpha", "beta"}},
		{"value", map[string]any{"@value": "alpha", "@language": "en"}, []string{"alpha"}},
		{"list", map[string]any{"@list": []any{"alpha", map[string]any{"@value": "beta"}}}, []string{"alpha", "beta"}},
		{"set", map[string]any{"@set": []any{"alpha", "beta"}}, []string{"alpha", "beta"}},
		{"array-list", []any{map[string]any{"@list": []any{"alpha", "beta"}}}, []string{"alpha", "beta"}},
		{"id", map[string]any{"@id": "alpha"}, []string{"alpha"}},
		{"id-alias", map[string]any{"id": "alpha"}, []string{"alpha"}},
		{"object-name", map[string]any{"schema:name": "alpha"}, []string{"alpha"}},
		{"id-before-name", map[string]any{"@id": "alpha", "name": "beta"}, []string{"alpha"}},
		{"empty-id", map[string]any{"@id": "", "name": "beta"}, []string{"beta"}},
		{"empty-list", map[string]any{"@list": []any{}, "name": "beta"}, nil},
		{"empty-object", map[string]any{}, nil},
	}
	total := 0
	for _, v := range []string{"2.0", "3.0", "master"} {
		ctx := contexts(t, v)
		props := properties(ctx)
		t.Logf("%s: %d properties", v, len(props))
		for _, term := range props {
			full := iri(ctx, term)
			keys := []string{term, full}
			if strings.Contains(full, "schema.org/") {
				keys = append(keys, "https://schema.org/"+strings.Split(full, "schema.org/")[1], "http://schema.org/"+strings.Split(full, "schema.org/")[1], "schema:"+strings.Split(full, "schema.org/")[1])
			} else {
				base, _ := ctx["codemeta"].(string)
				keys = append(keys, "codemeta:"+strings.TrimPrefix(full, base))
			}
			for _, key := range keys {
				for _, c := range cases {
					d := parse(t, map[string]any{"@context": contextURL(v), key: c.value})
					total++
					if got := d.Strings(term); !reflect.DeepEqual(got, c.want) {
						t.Errorf("%s/%s/%s/%s got %v want %v", v, term, key, c.name, got, c.want)
					}
					if !reflect.DeepEqual(d.Get(term), d.Get(key)) {
						t.Errorf("%s/%s alias mismatch", v, key)
					}
					a := parse(t, map[string]any{"@context": contextURL(v), "author": map[string]any{key: c.value}}).Author()[0]
					if got := a.Strings(term); !reflect.DeepEqual(got, c.want) {
						t.Errorf("agent %s/%s/%s/%s got %v want %v", v, term, key, c.name, got, c.want)
					}
				}
			}
		}
	}
	t.Logf("%d property/alias/shape checks", total)
}
func TestAllDeclaredRanges(t *testing.T) {
	data, e := os.ReadFile("internal/contexts/properties_description.csv")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	ranges := map[string][]string{}
	for _, r := range rows[1:] {
		p, _, _ := strings.Cut(r[0], ":")
		key := p + ":" + r[1]
		ranges[key] = append(ranges[key], strings.Split(r[2], " or ")...)
	}
	examples := map[string]any{"Text": "example", "URL": "https://example.org/item", "Date": "2024-02-29", "Datetime": "2024-02-29T12:00:00Z", "Integer": 12, "Number": 1.25, "Boolean": true}
	total := 0
	noRange := map[string][]string{}
	for _, v := range []string{"2.0", "3.0", "master"} {
		ctx := contexts(t, v)
		for _, term := range properties(ctx) {
			full := iri(ctx, term)
			key := rangeKey(ctx, full)
			if len(ranges[key]) == 0 {
				noRange[v] = append(noRange[v], term)
			}
			for _, typ := range ranges[key] {
				base, scalar := examples[typ]
				if !scalar {
					base = map[string]any{"@id": "https://example.org/item"}
				}
				shapes := []any{base, []any{base}, map[string]any{"@set": []any{base}}, map[string]any{"@list": []any{base}}}
				if scalar {
					shapes = append(shapes, map[string]any{"@value": base})
				}
				for i, val := range shapes {
					d := parse(t, map[string]any{"@context": contextURL(v), term: val})
					total++
					if ds := d.Validate(); len(ds) > 0 {
						t.Errorf("%s/%s/%s/shape%d: %+v", v, term, typ, i, ds)
					}
				}
			}
		}
	}
	t.Logf("%d declared-range shape checks; properties using supplemental ranges: %v", total, noRange)
}
func agents(d *cm.Document, rel string) []cm.Agent {
	switch rel {
	case "author":
		return d.Author()
	case "contributor":
		return d.Contributor()
	case "maintainer":
		return d.Maintainer()
	case "copyrightHolder":
		return d.CopyrightHolder()
	case "funder":
		return d.Funder()
	}
	return nil
}
func TestAgentContainers(t *testing.T) {
	a := map[string]any{"@type": "Person", "givenName": "Ada"}
	b := map[string]any{"@type": "Person", "givenName": "Grace"}
	shapes := []struct {
		name  string
		value any
	}{
		{"array", []any{a, b}}, {"list", map[string]any{"@list": []any{a, b}}}, {"set", map[string]any{"@set": []any{a, b}}},
		{"array-list", []any{map[string]any{"@list": []any{a, b}}}}, {"array-set", []any{map[string]any{"@set": []any{a, b}}}},
	}
	for _, v := range []string{"2.0", "3.0", "master"} {
		for _, rel := range []string{"author", "contributor", "maintainer", "copyrightHolder", "funder"} {
			for _, s := range shapes {
				for _, role := range []bool{false, true} {
					label := v + "/" + rel + "/" + s.name
					if role {
						label += "/role"
					}
					t.Run(label, func(t *testing.T) {
						checkAgentContainer(t, v, rel, s.value, role)
					})
				}
			}
		}
	}
}

func checkAgentContainer(t *testing.T, version, relation string, value any, role bool) {
	t.Helper()
	if role {
		value = map[string]any{"@type": "schema:Role", relation: value}
	}
	doc := parse(t, map[string]any{"@context": contextURL(version), relation: value})
	if ds := doc.Validate(); len(ds) != 0 {
		t.Fatalf("validation: %+v", ds)
	}
	got := agents(doc, relation)
	if role {
		if len(got) != 1 {
			t.Fatal("missing role")
		}
		got = got[0].Agents()
	}
	var names []string
	for _, agent := range got {
		names = append(names, agent.GivenName())
	}
	if !reflect.DeepEqual(names, []string{"Ada", "Grace"}) {
		t.Errorf("names=%q", names)
	}
}
func TestTextAccessorShapes(t *testing.T) {
	shapes := []struct {
		name string
		wrap func(string) any
	}{
		{"scalar", func(v string) any { return v }}, {"array", func(v string) any { return []any{v} }},
		{"value", func(v string) any { return map[string]any{"@value": v} }}, {"list", func(v string) any { return map[string]any{"@list": []any{v}} }}, {"set", func(v string) any { return map[string]any{"@set": []any{v}} }},
	}
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			d := parse(t, map[string]any{"@context": v3, "name": s.wrap("Name"), "description": s.wrap("Description"), "version": s.wrap("1.2"), "author": map[string]any{"@type": "Person", "name": s.wrap("Author"), "givenName": s.wrap("Ada"), "familyName": s.wrap("Lovelace")}, "contributor": map[string]any{"@type": "Role", "roleName": s.wrap("developer"), "contributor": map[string]any{"name": "A"}}})
			if ds := d.Validate(); len(ds) > 0 {
				t.Fatal(ds)
			}
			a := d.Author()[0]
			values := map[string]string{"doc.Name": d.Name(), "doc.Description": d.Description(), "version.Strings": strings.Join(d.Strings("version"), ", "), "agent.Name": a.Name(), "agent.GivenName": a.GivenName(), "agent.FamilyName": a.FamilyName(), "roleName.Strings": strings.Join(d.Contributor()[0].Strings("roleName"), ", ")}
			for name, v := range values {
				if v == "" {
					t.Errorf("%s empty for valid %s", name, s.name)
				}
			}
		})
	}
}
func TestContextPropagation(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  []string
	}{
		{"local-enable", map[string]any{"programmingLanguage": map[string]any{"@context": v3, "name": map[string]any{"schema:name": "Go"}}}, []string{"Go"}},
		{"local-reset", map[string]any{"@context": v3, "programmingLanguage": map[string]any{"@context": nil, "name": map[string]any{"schema:name": "Go"}}}, nil},
		{"local-modified", map[string]any{"@context": v3, "programmingLanguage": map[string]any{"@context": map[string]any{"schema": "https://example.org/"}, "name": map[string]any{"schema:name": "Go"}}}, nil},
		{"direct-local-enable", map[string]any{"programmingLanguage": map[string]any{"@context": v3, "schema:name": "Go"}}, []string{"Go"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := parse(t, c.input)
			if got := d.Strings("programmingLanguage"); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v want %v; diagnostics=%+v", got, c.want, d.Validate())
			}
		})
	}
}
func TestNestedContextArrays(t *testing.T) {
	for _, local := range []any{map[string]any{}, []any{map[string]any{}}, map[string]any{"givenName": "schema:givenName"}, []any{map[string]any{"givenName": "schema:givenName"}}} {
		d := parse(t, map[string]any{"@context": v3, "author": map[string]any{"@context": local, "schema:givenName": "Ada"}})
		if got := d.Author()[0].GivenName(); got != "Ada" {
			t.Errorf("local=%v got %q diagnostics=%+v", local, got, d.Validate())
		}
	}
}
func TestReferenceValidation(t *testing.T) {
	for _, term := range []string{"codeRepository", "downloadUrl", "installUrl", "url", "issueTracker", "referencePublication", "license", "author"} {
		for _, key := range []string{"@id", "id"} {
			d := parse(t, map[string]any{"@context": v3, term: map[string]any{key: "not an IRI"}})
			if ds := d.Validate(); len(ds) == 0 {
				t.Errorf("%s/%s accepts invalid IRI", term, key)
			}
		}
	}
}

func TestRawAccessorsRetainAllShapes(t *testing.T) {
	values := []any{nil, "", "text", json.Number("1.20"), true, []any{"first", "second"}, map[string]any{"@value": "text"}, map[string]any{"@list": []any{"first", "second"}}, map[string]any{"@id": "urn:example:x"}, map[string]any{"@set": []any{"first", "second"}}}
	accessors := map[string]func(*cm.Document) cm.Value{"codeRepository": (*cm.Document).CodeRepository, "version": (*cm.Document).SoftwareVersion, "license": (*cm.Document).License, "keywords": (*cm.Document).Keywords, "programmingLanguage": (*cm.Document).ProgrammingLanguages, "datePublished": (*cm.Document).DatePublished, "dateModified": (*cm.Document).DateModified, "developmentStatus": (*cm.Document).DevelopmentStatus, "identifier": (*cm.Document).Identifier}
	count := 0
	for _, v := range []string{"2.0", "3.0", "master"} {
		ctx := contexts(t, v)
		for term, accessor := range accessors {
			for _, key := range []string{term, iri(ctx, term)} {
				for _, value := range values {
					d := parse(t, map[string]any{"@context": contextURL(v), key: value})
					count++
					if !reflect.DeepEqual(accessor(d), d.Get(key)) {
						t.Errorf("%s/%s did not retain value", v, key)
					}
				}
			}
		}
	}
	t.Logf("%d raw-accessor preservation checks", count)
}

func TestGenericScopesEveryField(t *testing.T) {
	for _, v := range []string{"2.0", "3.0", "master"} {
		for _, term := range properties(contexts(t, v)) {
			for _, s := range []struct {
				name     string
				local    any
				valueKey string
				want     []string
			}{
				{"inherit-empty", map[string]any{}, "schema:name", []string{"Name"}},
				{"inherit-partial", map[string]any{"name": "schema:name"}, "schema:name", []string{"Name"}},
				{"reset", nil, "schema:name", nil},
				{"unsupported", "https://example.org/context", "schema:name", nil},
				{"modified", map[string]any{"schema": "https://example.org/"}, "schema:name", nil},
				{"replace", v3, "schema:name", []string{"Name"}},
			} {
				d := parse(t, map[string]any{"@context": contextURL(v), term: map[string]any{"@context": s.local, s.valueKey: "Name"}})
				if got := d.Strings(term); !reflect.DeepEqual(got, s.want) {
					t.Errorf("%s/%s/%s got %v want %v", v, term, s.name, got, s.want)
				}
			}
		}
	}
}

func TestAllRangeInvalidShapes(t *testing.T) {
	data, e := os.ReadFile("internal/contexts/properties_description.csv")
	if e != nil {
		t.Fatal(e)
	}
	rows, e := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	ranges := map[string]string{}
	for _, r := range rows[1:] {
		p, _, _ := strings.Cut(r[0], ":")
		ranges[p+":"+r[1]] += " " + r[2]
	}
	count := 0
	for _, v := range []string{"2.0", "3.0", "master"} {
		ctx := contexts(t, v)
		for _, term := range properties(ctx) {
			full := iri(ctx, term)
			key := rangeKey(ctx, full)
			expected := ranges[key]
			bad := any(true)
			if strings.Contains(expected, "Boolean") {
				bad = 12
			}
			for _, value := range []any{bad, []any{bad}, map[string]any{"@value": bad}, map[string]any{"@list": []any{bad}}, map[string]any{"@set": []any{bad}}} {
				d := parse(t, map[string]any{"@context": contextURL(v), term: value})
				count++
				ds := d.Validate()
				if len(ds) != 1 || ds[0].Code != "value_type" {
					t.Errorf("%s/%s invalid value: %+v", v, term, ds)
				}
			}
		}
	}
	t.Logf("%d invalid-range shape checks", count)
}

func rangeKey(ctx map[string]any, full string) string {
	if strings.Contains(full, "schema.org/") {
		return "schema:" + strings.Split(full, "schema.org/")[1]
	}
	base, _ := ctx["codemeta"].(string)
	return "codemeta:" + strings.TrimPrefix(full, base)
}

func TestAgentClassificationEmptyFields(t *testing.T) {
	for _, rel := range []string{"author", "contributor", "maintainer", "copyrightHolder", "funder"} {
		for _, field := range []string{"givenName", "familyName", "affiliation"} {
			for _, empty := range []any{nil, []any{}} {
				d := parse(t, map[string]any{"@context": v3, rel: map[string]any{"@type": "Organization", "name": "Team", field: empty}})
				if ds := d.Validate(); len(ds) != 0 {
					t.Errorf("%s/%s/%v: %+v", rel, field, empty, ds)
				}
				if got := agents(d, rel)[0].Kind(); got != cm.AgentOrganization {
					t.Errorf("%s/%s/%v: kind=%d", rel, field, empty, got)
				}
			}
		}
		for _, empty := range []any{nil, []any{}} {
			d := parse(t, map[string]any{"@context": v3, rel: map[string]any{"@type": "Person", "name": "Ada", "roleName": empty}})
			if got := agents(d, rel)[0].Kind(); got != cm.AgentPerson {
				t.Errorf("%s/null-role/%v kind=%d", rel, empty, got)
			}
		}
		d := parse(t, map[string]any{"@context": v3, rel: map[string]any{"@context": map[string]any{}, "@id": "https://example.org/person"}})
		if got := agents(d, rel)[0].Kind(); got != cm.AgentReference {
			t.Errorf("%s/context-reference kind=%d diagnostics=%+v", rel, got, d.Validate())
		}
	}
}

func TestNullLiteralLanguage(t *testing.T) {
	for _, term := range []string{"name", "description", "keywords", "givenName", "familyName", "roleName"} {
		for _, keyword := range []string{"@language", "@direction"} {
			d := parse(t, map[string]any{"@context": v3, term: map[string]any{"@value": "Value", keyword: nil}})
			if ds := d.Validate(); len(ds) > 0 {
				t.Errorf("%s/%s: %+v", term, keyword, ds)
			}
			if got := d.Strings(term); !reflect.DeepEqual(got, []string{"Value"}) {
				t.Errorf("projection lost for %s", term)
			}
		}
	}
}

func TestNestedAgentContextShapes(t *testing.T) {
	for _, rel := range []string{"author", "contributor", "maintainer", "copyrightHolder", "funder"} {
		for _, key := range []string{"name", "givenName", "familyName", "roleName", "affiliation", "identifier", "email", "url"} {
			child := map[string]any{"@context": map[string]any{}, "schema:" + key: "Value"}
			if rel == "maintainer" {
				child["@type"] = "Person"
			}
			d := parse(t, map[string]any{"@context": v3, rel: child})
			a := agents(d, rel)[0]
			if a.Get(key).Text() != "Value" {
				t.Errorf("%s/%s missing", rel, key)
			}
		}
	}
}
