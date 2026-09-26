package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedTablesMatchPinnedSources(t *testing.T) {
	output := filepath.Join(t.TempDir(), "terms.go")
	if err := run("../contexts", output); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../terms_generated.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("generated tables differ; run go generate ./...")
	}
}
func TestRejectUnsupportedContextConstructs(t *testing.T) {
	for _, input := range []string{
		`{"@context":{"@vocab":"https://example.org/"}}`,
		`{"@context":{"name":{"@id":"schema:name","@reverse":true}}}`,
		`{"@context":{"name":{"@id":"schema:name","@container":"@language"}}}`,
		`{"@context":{"name":null}}`,
		`{"@context":{"name":{"@id":"schema:name","@context":{}}}}`,
		`{"@context":{},"other":1}`,
	} {
		if _, _, err := readContext([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}
func TestRejectUnknownPropertyExpressions(t *testing.T) {
	for _, expression := range []string{"Any", "Text, URL", "Text or Unknown", "Text and URL", "Text or"} {
		if _, err := translate(expression); err == nil {
			t.Fatalf("accepted %q", expression)
		}
	}
	if _, err := readRanges(strings.NewReader("Parent Type,Property,Type,Description\nschema:Thing,name,Whatever,Description\n")); err == nil {
		t.Fatal("accepted unknown CSV range")
	}
}
