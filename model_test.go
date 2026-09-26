package codemeta_test

import (
	"reflect"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestAgents(t *testing.T) {
	doc, err := codemeta.Parse([]byte(`{"@context":"https://w3id.org/codemeta/3.0","name":"Example","version":2.0,"author":["A",{"@id":"https://example.org/a"},{"givenName":"B","familyName":"C"},{"type":"Organization","name":"D"},{"@type":"schema:Role","roleName":"developer","author":{"givenName":"E"}},{"type":"Organization","givenName":"F"}],"maintainer":{"givenName":"G"},"keywords":"one"}`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name() != "Example" || doc.Version() != codemeta.Version3 || doc.SoftwareVersion().Kind() != codemeta.Number || doc.SoftwareVersion().Text() != "2.0" {
		t.Fatal("metadata changed")
	}
	kinds := []codemeta.AgentKind{codemeta.AgentText, codemeta.AgentReference, codemeta.AgentPerson, codemeta.AgentOrganization, codemeta.AgentRole, codemeta.AgentConflict}
	authors := doc.Author()
	if len(authors) != len(kinds) {
		t.Fatal(authors)
	}
	for i, want := range kinds {
		if authors[i].Kind() != want {
			t.Fatalf("author %d = %v, want %v", i, authors[i].Kind(), want)
		}
	}
	if authors[4].RoleName().Text() != "developer" || authors[4].Agents()[0].GivenName() != "E" {
		t.Fatal("role lost")
	}
	if doc.Get("maintainer").Kind() != codemeta.Object || len(doc.Maintainer()) != 1 || doc.Maintainer()[0].GivenName() != "G" {
		t.Fatal("scalar agent changed")
	}
	if doc.Keywords().Kind() != codemeta.String || len(doc.Keywords().Values()) != 1 {
		t.Fatal("scalar keywords changed")
	}
	authors[0] = codemeta.Agent{}
	if doc.Author()[0].Value().Text() != "A" {
		t.Fatal("mutable agent slice")
	}
}
func TestResolvedAccessors(t *testing.T) {
	doc, err := codemeta.Parse([]byte(`{"@context":"https://w3id.org/codemeta/3.0","schema:name":"N","http://schema.org/description":"D","schema:author":{"schema:givenName":"Ada","id":"https://example.org/ada"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name() != "N" || doc.Description() != "D" || doc.Get("https://schema.org/name").Text() != "N" {
		t.Fatal("term resolution")
	}
	if doc.Author()[0].GivenName() != "Ada" || doc.Author()[0].Identifier().Text() != "https://example.org/ada" {
		t.Fatal("agent resolution")
	}
	if ds := doc.Validate(); len(ds) != 0 {
		t.Fatal(ds)
	}
}

func TestMetadataViews(t *testing.T) {
	doc, err := codemeta.Parse([]byte(`{"@context":"https://w3id.org/codemeta/3.0","name":"Example","description":"Description","codeRepository":"https://example.org/repo","version":"2.0","license":"https://spdx.org/licenses/MIT","keywords":["metadata"],"programmingLanguage":{"name":"Go"},"datePublished":"2024-02-29","dateModified":"2024-03-01","developmentStatus":"active","identifier":"https://example.org/id","author":{"name":"Authors"},"contributor":{"name":"Contributors"},"maintainer":{"givenName":"Ada"},"copyrightHolder":{"name":"Holders"},"funder":{"name":"Funders"}}`))
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]codemeta.Value{
		"codeRepository": doc.CodeRepository(), "version": doc.SoftwareVersion(), "license": doc.License(), "keywords": doc.Keywords(), "programmingLanguage": doc.ProgrammingLanguages(), "datePublished": doc.DatePublished(), "dateModified": doc.DateModified(), "developmentStatus": doc.DevelopmentStatus(), "identifier": doc.Identifier(),
	}
	for name, value := range fields {
		if !reflect.DeepEqual(value, doc.Get(name)) {
			t.Fatalf("view differs for %s", name)
		}
	}
	for name, agents := range map[string][]codemeta.Agent{"author": doc.Author(), "contributor": doc.Contributor(), "maintainer": doc.Maintainer(), "copyrightHolder": doc.CopyrightHolder(), "funder": doc.Funder()} {
		if len(agents) != 1 || !reflect.DeepEqual(agents[0].Value(), doc.Get(name)) {
			t.Fatalf("agent view differs for %s", name)
		}
	}
	if doc.Name() != "Example" || doc.Description() != "Description" {
		t.Fatal("text metadata")
	}
	if ds := doc.Validate(); len(ds) != 0 {
		t.Fatal(ds)
	}
}
