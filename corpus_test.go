package codemeta_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/git-pkgs/codemeta"
)

type corpusFixture struct {
	File           string                `json:"file"`
	Repository     string                `json:"repository"`
	Path           string                `json:"path"`
	Revision       string                `json:"revision"`
	Commit         string                `json:"commit"`
	Blob           string                `json:"blob"`
	SHA256         string                `json:"sha256"`
	Compact        bool                  `json:"compact"`
	Parse          string                `json:"parse"`
	Version        codemeta.Version      `json:"version"`
	Name           string                `json:"name"`
	Diagnostics    []codemeta.Diagnostic `json:"diagnostics"`
	Classification string                `json:"classification"`
}

func corpusManifest(t testing.TB, group string) []corpusFixture {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", group, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []corpusFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("empty corpus")
	}
	return fixtures
}
func TestCorpus(t *testing.T) {
	for _, group := range []string{"upstream", "comparisons", "real-world"} {
		for _, fixture := range corpusManifest(t, group) {
			if group == "real-world" && !fixture.Compact && os.Getenv("CODEMETA_CORPUS") != "all" {
				continue
			}
			t.Run(group+"/"+fixture.File, func(t *testing.T) { checkCorpusFixture(t, group, fixture) })
		}
	}
}
func checkCorpusFixture(t *testing.T, group string, fixture corpusFixture) {
	t.Helper()
	if fixture.Repository == "" || fixture.Path == "" || fixture.Blob == "" || (fixture.Commit == "" && fixture.Revision == "") {
		t.Fatal("missing provenance")
	}
	data, err := os.ReadFile(filepath.Join("testdata", group, fixture.File))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != fixture.SHA256 {
		t.Fatal("fixture bytes changed")
	}
	doc, err := codemeta.Parse(data)
	if fixture.Parse != "ok" {
		var detail *codemeta.Error
		if !errors.As(err, &detail) || detail.Code != fixture.Parse {
			t.Fatalf("parse=%v, want %s", err, fixture.Parse)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version() != fixture.Version || doc.Name() != fixture.Name {
		t.Fatalf("metadata: version %s, name %s", doc.Version(), doc.Name())
	}
	got := doc.Validate()
	if len(got) != len(fixture.Diagnostics) || (len(got) > 0 && !reflect.DeepEqual(got, fixture.Diagnostics)) {
		t.Fatalf("diagnostics:\n%+v\nwant:\n%+v", got, fixture.Diagnostics)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var original map[string]any
	if err := decoder.Decode(&original); err != nil {
		t.Fatal(err)
	}
	for key, value := range original {
		checkJSONValue(t, doc.Get(key), value)
	}
}
func checkJSONValue(t *testing.T, got codemeta.Value, want any) {
	t.Helper()
	switch v := want.(type) {
	case nil:
		if got.Kind() != codemeta.Null {
			t.Fatal("null lost")
		}
	case string:
		if got.Kind() != codemeta.String || got.Text() != v {
			t.Fatalf("string = %q, want %q", got.Text(), v)
		}
	case json.Number:
		if got.Kind() != codemeta.Number || got.Text() != string(v) {
			t.Fatal("number spelling changed")
		}
	case bool:
		if got.Kind() != codemeta.Boolean || (got.Text() == "true") != v {
			t.Fatal("boolean changed")
		}
	case []any:
		items := got.Items()
		if got.Kind() != codemeta.Array || len(items) != len(v) {
			t.Fatal("array changed")
		}
		for i, item := range v {
			checkJSONValue(t, items[i], item)
		}
	case map[string]any:
		if got.Kind() != codemeta.Object || len(got.Fields()) != len(v) {
			t.Fatal("object changed")
		}
		for key, item := range v {
			checkJSONValue(t, got.Get(key), item)
		}
	default:
		t.Fatalf("unexpected reference type %T", want)
	}
}
func TestCorpusDefaultLimits(t *testing.T) {
	for _, group := range []string{"upstream", "comparisons", "real-world"} {
		for _, fixture := range corpusManifest(t, group) {
			if group == "real-world" && !fixture.Compact && os.Getenv("CODEMETA_CORPUS") != "all" {
				continue
			}
			data, err := os.ReadFile(filepath.Join("testdata", group, fixture.File))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := codemeta.Parse(data)
			if errors.Is(err, codemeta.ErrLimit) {
				t.Fatalf("default limits exclude %s", fixture.Repository)
			}
			if doc != nil {
				for _, d := range doc.Validate() {
					if d.Code == "diagnostic_limit" {
						t.Fatalf("default diagnostics truncate %s", fixture.Repository)
					}
				}
			}
		}
	}
}
func BenchmarkCorpus(b *testing.B) {
	for _, fixture := range corpusManifest(b, "real-world") {
		if !fixture.Compact && os.Getenv("CODEMETA_CORPUS") != "all" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("testdata", "real-world", fixture.File))
		if err != nil {
			b.Fatal(err)
		}
		name := strings.TrimPrefix(fixture.Repository, "https://github.com/")
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				doc, err := codemeta.Parse(data)
				if err == nil {
					_ = doc.Validate()
				}
			}
		})
	}
}
