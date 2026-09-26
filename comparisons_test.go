package codemeta_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestDeliberateGeneratorDifferences(t *testing.T) {
	data, err := os.ReadFile("testdata/comparisons/differences.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name           string
		Input          json.RawMessage
		ExpectedCodes  []string `json:"expected_codes"`
		Reason         string
		GeneratorValid bool `json:"generator_valid"`
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 4 {
		t.Fatal("missing comparison cases")
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			doc, err := codemeta.Parse(fixture.Input)
			if err != nil {
				t.Fatal(err)
			}
			codes := []string{}
			for _, d := range doc.Validate() {
				codes = append(codes, d.Code)
			}
			if !reflect.DeepEqual(codes, fixture.ExpectedCodes) {
				t.Fatalf("%v, want %v", codes, fixture.ExpectedCodes)
			}
			if (len(codes) == 0) == fixture.GeneratorValid || fixture.Reason == "" {
				t.Fatal("expected a documented difference")
			}
		})
	}
}
