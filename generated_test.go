package codemeta_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestPinnedInputChecksums(t *testing.T) {
	data, err := os.ReadFile("internal/contexts/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	var sources []struct {
		File   string
		SHA256 string
	}
	if err := json.Unmarshal(data, &sources); err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		data, err := os.ReadFile(source.File)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != source.SHA256 {
			t.Errorf("modified pinned input %s", source.File)
		}
	}
}
func TestEveryContextTerm(t *testing.T) {
	for _, version := range []string{"2.0", "3.0", "master"} {
		data, err := os.ReadFile("internal/contexts/" + version + ".jsonld")
		if err != nil {
			t.Fatal(err)
		}
		var pinned struct {
			Context map[string]any `json:"@context"`
		}
		if err := json.Unmarshal(data, &pinned); err != nil {
			t.Fatal(err)
		}
		wantCount := map[string]int{"2.0": 74, "3.0": 83, "master": 94}[version]
		if len(pinned.Context) != wantCount {
			t.Fatalf("%s: %d terms", version, len(pinned.Context))
		}
		for term := range pinned.Context {
			t.Run(version+"/"+term, func(t *testing.T) { checkGeneratedTerm(t, pinned.Context, term) })
		}
	}
}

func checkGeneratedTerm(t *testing.T, context map[string]any, term string) {
	t.Helper()
	var value any
	if term == "type" {
		value = "SoftwareSourceCode"
	}
	if term == "id" {
		value = "https://example.org/project"
	}
	encoded, err := json.Marshal(map[string]any{"@context": context, term: value})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := codemeta.Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if ds := doc.Validate(); len(ds) != 0 {
		t.Fatal(ds)
	}
}

func TestGeneratedPropertyRanges(t *testing.T) {
	data, err := os.ReadFile("internal/contexts/properties_description.csv")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	expressions := map[string][]string{}
	for _, row := range rows[1:] {
		prefix, _, _ := strings.Cut(row[0], ":")
		key := prefix + ":" + row[1]
		expressions[key] = append(expressions[key], row[2])
	}
	for _, version := range []string{"2.0", "3.0", "master"} {
		data, err := os.ReadFile("internal/contexts/" + version + ".jsonld")
		if err != nil {
			t.Fatal(err)
		}
		var pinned struct {
			Context map[string]any `json:"@context"`
		}
		if err := json.Unmarshal(data, &pinned); err != nil {
			t.Fatal(err)
		}
		for name, definition := range pinned.Context {
			def, ok := definition.(map[string]any)
			if !ok {
				continue
			}
			iri, _ := def["@id"].(string)
			expected := expressions[iri]
			if len(expected) == 0 {
				continue
			}
			t.Run(version+"/"+name, func(t *testing.T) { checkPropertyRange(t, pinned.Context, name, expected) })
		}
	}
}
func checkPropertyRange(t *testing.T, context map[string]any, name string, expressions []string) {
	t.Helper()
	valid := rangeExample(expressions[0])
	invalid := any(true)
	if strings.Contains(strings.Join(expressions, " "), "Boolean") {
		invalid = 12
	}
	for _, tc := range []struct {
		value any
		valid bool
	}{{valid, true}, {invalid, false}} {
		input, err := json.Marshal(map[string]any{"@context": context, name: tc.value})
		if err != nil {
			t.Fatal(err)
		}
		doc, err := codemeta.Parse(input)
		if err != nil {
			t.Fatalf("representable value failed parsing: %v", err)
		}
		ds := doc.Validate()
		if tc.valid && len(ds) != 0 {
			t.Fatalf("valid range example: %+v", ds)
		}
		if !tc.valid && (len(ds) != 1 || ds[0].Code != "value_type" || ds[0].Path != name) {
			t.Fatalf("invalid range example: %+v", ds)
		}
	}
}
func rangeExample(expression string) any {
	switch strings.Split(expression, " or ")[0] {
	case "Text":
		return "example"
	case "URL":
		return "https://example.org/item"
	case "Date":
		return "2024-02-29"
	case "Datetime":
		return "2024-02-29T12:00:00Z"
	case "Number", "Integer":
		return 12
	case "Boolean":
		return true
	default:
		return map[string]any{"@id": "https://example.org/item"}
	}
}
