package codemeta_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/git-pkgs/codemeta"
)

func TestJSONConformance(t *testing.T) {
	cases := []struct {
		name, input, code string
		category          error
	}{
		{"empty", " \n", "empty_input", codemeta.ErrSyntax},
		{"root array", "[]", "root_type", codemeta.ErrType},
		{"root null", "null", "root_type", codemeta.ErrType},
		{"duplicate", `{"name":1,"n\u0061me":2}`, "duplicate_key", codemeta.ErrSyntax},
		{"trailing comma", `{"a":1,}`, "syntax", codemeta.ErrSyntax},
		{"array comma", `{"a":[1,]}`, "syntax", codemeta.ErrSyntax},
		{"comment", `{"a":/*x*/1}`, "syntax", codemeta.ErrSyntax},
		{"nan", `{"a":NaN}`, "syntax", codemeta.ErrSyntax},
		{"infinity", `{"a":Infinity}`, "syntax", codemeta.ErrSyntax},
		{"leading zero", `{"a":01}`, "syntax", codemeta.ErrSyntax},
		{"fraction", `{"a":1.}`, "syntax", codemeta.ErrSyntax},
		{"exponent", `{"a":1e+}`, "syntax", codemeta.ErrSyntax},
		{"high surrogate", `{"a":"\ud800"}`, "lone_surrogate", codemeta.ErrSyntax},
		{"low surrogate", `{"a":"\udc00"}`, "lone_surrogate", codemeta.ErrSyntax},
		{"bad pair", `{"a":"\ud800\u0061"}`, "lone_surrogate", codemeta.ErrSyntax},
		{"nul", "{\"a\":\"\x00\"}", "syntax", codemeta.ErrSyntax},
		{"utf8", "{\"a\":\"\xff\"}", "syntax", codemeta.ErrSyntax},
		{"bom", "\xef\xbb\xbf{}", "bom", codemeta.ErrUnsupported},
		{"extra", "{}{}", "syntax", codemeta.ErrSyntax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := codemeta.Parse([]byte(tc.input))
			var detail *codemeta.Error
			if !errors.Is(err, tc.category) || !errors.As(err, &detail) {
				t.Fatalf("error = %v", err)
			}
			if detail.Code != tc.code || detail.Line < 1 || detail.Column < 1 {
				t.Fatalf("detail = %+v", detail)
			}
		})
	}
}

func TestJSONPreservation(t *testing.T) {
	input := []byte("{\r\n\"é\": [123456789012345678901234567890, -0, 2.0, 1E+20],\r\n\"text\":\"a\\u0000\\ud83d\\ude00\\n\",\"null\":null}")
	doc, err := codemeta.Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"123456789012345678901234567890", "-0", "2.0", "1E+20"} {
		got := doc.Get("é").Items()[i]
		if got.Kind() != codemeta.Number || got.Text() != want {
			t.Fatalf("number = %+v", got)
		}
	}
	if got := doc.Get("é").Position(); got != (codemeta.Position{Line: 2, Column: 6}) {
		t.Fatalf("position = %+v", got)
	}
	if doc.Get("text").Text() != "a\x00😀\n" {
		t.Fatalf("text = %q", doc.Get("text").Text())
	}
	if doc.Get("null").Kind() != codemeta.Null || doc.Get("absent").Kind() != codemeta.Missing {
		t.Fatal("null and missing conflated")
	}
	if len(doc.Get("null").Values()) != 1 || len(doc.Get("absent").Values()) != 0 {
		t.Fatal("scalar flattening")
	}
	for i := range input {
		input[i] = 0
	}
	fields := doc.Fields()
	fields[0].Name = "changed"
	items := doc.Get("é").Items()
	items[0] = codemeta.Value{}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				if doc.Get("é").Items()[0].Text() != "123456789012345678901234567890" {
					t.Error("retained data changed")
				}
			}
		})
	}
	wg.Wait()
}

func TestParseLimits(t *testing.T) {
	cases := []struct {
		input   string
		options codemeta.ParseOptions
		code    string
	}{
		{`{}`, codemeta.ParseOptions{MaxBytes: 1}, "byte_limit"},
		{`{"x":[]}`, codemeta.ParseOptions{MaxDepth: 1}, "depth_limit"},
		{`{"x":null}`, codemeta.ParseOptions{MaxNodes: 2}, "node_limit"},
		{`{"x":"é"}`, codemeta.ParseOptions{MaxStringBytes: 2}, "string_limit"},
		{`{"x":123}`, codemeta.ParseOptions{MaxStringBytes: 3}, "string_limit"},
	}
	for _, tc := range cases {
		_, err := codemeta.ParseWithOptions([]byte(tc.input), tc.options)
		var detail *codemeta.Error
		if !errors.Is(err, codemeta.ErrLimit) || !errors.As(err, &detail) || detail.Code != tc.code {
			t.Fatalf("%s: %v", tc.code, err)
		}
	}
	for _, opts := range []codemeta.ParseOptions{{MaxBytes: -1}, {MaxDepth: -1}, {MaxNodes: -1}, {MaxStringBytes: -1}, {MaxDiagnostics: -1}} {
		if _, err := codemeta.ParseWithOptions([]byte(`{}`), opts); !errors.Is(err, codemeta.ErrOptions) {
			t.Fatal(err)
		}
	}
	for _, input := range []string{`{}`, `{"x":null}`, `{"x":"é"}`} {
		if _, err := codemeta.Read(strings.NewReader(input), codemeta.ParseOptions{MaxBytes: int64(len(input)), MaxDepth: 2, MaxNodes: 3, MaxStringBytes: 3}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := codemeta.Parse([]byte(`{"x":` + strings.Repeat("[", 64) + "0" + strings.Repeat("]", 64) + "}"))
	if !errors.Is(err, codemeta.ErrLimit) {
		t.Fatal(err)
	}
}

type failingReader struct{ cause error }

func (r failingReader) Read(p []byte) (int, error) { return copy(p, `{}`), r.cause }

type shortReader struct {
	r      io.Reader
	closed bool
}

func (r *shortReader) Read(p []byte) (int, error) { return r.r.Read(p[:1]) }
func (r *shortReader) Close() error               { r.closed = true; return nil }

func TestReaderAndFile(t *testing.T) {
	cause := errors.New("read failure")
	_, err := codemeta.Read(failingReader{cause}, codemeta.ParseOptions{})
	if !errors.Is(err, cause) || !errors.Is(err, codemeta.ErrIO) {
		t.Fatal(err)
	}
	r := &shortReader{r: strings.NewReader(`{"name":"example"}`)}
	doc, err := codemeta.Read(r, codemeta.ParseOptions{})
	if err != nil || r.closed || doc.Get("name").Text() != "example" {
		t.Fatalf("Read = %v, %v", doc, err)
	}
	bounded := strings.NewReader(`{}    `)
	_, err = codemeta.Read(bounded, codemeta.ParseOptions{MaxBytes: 2})
	if !errors.Is(err, codemeta.ErrLimit) || bounded.Len() != 3 {
		t.Fatalf("limit: %v, remaining %d", err, bounded.Len())
	}
	path := filepath.Join(t.TempDir(), "codemeta.json")
	_, err = codemeta.ReadFile(path, codemeta.ParseOptions{})
	if !errors.Is(err, fs.ErrNotExist) || !errors.Is(err, codemeta.ErrIO) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"name":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	doc, err = codemeta.ReadFile(path, codemeta.ParseOptions{})
	if err != nil || doc.Get("name").Text() != "file" {
		t.Fatalf("ReadFile = %v, %v", doc, err)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{`{}`, `{"name":"é"}`, `{"author":[{"name":"A"}]}`, `{"x":"\ud800"}`, `{"x":1,"x":2}`} {
		f.Add([]byte(seed))
	}
	f.Add([]byte(`{"@context":"https://w3id.org/codemeta/3.0","@type":["SoftwareSourceCode"],"author":[{"givenName":"Ada"}],"datePublished":"2023-02-29","unknown":true}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		a, errA := codemeta.Parse(data)
		b, errB := codemeta.Parse(data)
		if (errA == nil) != (errB == nil) || !reflect.DeepEqual(a, b) {
			t.Fatal("nondeterministic parse")
		}
		if errA == nil && !json.Valid(data) {
			t.Fatal("accepted invalid JSON")
		}
		if errA == nil && !reflect.DeepEqual(a.Validate(), b.Validate()) {
			t.Fatal("nondeterministic diagnostics")
		}
	})
}
func FuzzRead(f *testing.F) {
	f.Add([]byte(`{"name":"example"}`))
	f.Add([]byte(`{"@context":"https://w3id.org/codemeta/3.0","author":{"@list":[{"@type":"Role","roleName":"developer","author":"Ada"}]}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		a, errA := codemeta.Parse(data)
		b, errB := codemeta.Read(bytes.NewReader(data), codemeta.ParseOptions{})
		if (errA == nil) != (errB == nil) || !reflect.DeepEqual(a, b) {
			t.Fatal("reader differs from Parse")
		}
		if errA == nil && !reflect.DeepEqual(a.Validate(), b.Validate()) {
			t.Fatal("reader diagnostics differ from Parse")
		}
	})
}
func BenchmarkParse(b *testing.B) {
	data := []byte(`{"@context":"https://w3id.org/codemeta/3.0","name":"example","author":[{"givenName":"Ada","familyName":"Lovelace"}]}`)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := codemeta.Parse(data); err != nil {
			b.Fatal(err)
		}
	}
}

func TestLimitsAtPublicBoundaries(t *testing.T) {
	input := []byte(`{"x":"é"}`)
	file := filepath.Join(t.TempDir(), "codemeta.json")
	if err := os.WriteFile(file, input, 0o600); err != nil {
		t.Fatal(err)
	}
	boundaries := map[string]func(codemeta.ParseOptions) (*codemeta.Document, error){
		"ParseWithOptions": func(o codemeta.ParseOptions) (*codemeta.Document, error) { return codemeta.ParseWithOptions(input, o) },
		"Read": func(o codemeta.ParseOptions) (*codemeta.Document, error) {
			return codemeta.Read(bytes.NewReader(input), o)
		},
		"ReadFile": func(o codemeta.ParseOptions) (*codemeta.Document, error) { return codemeta.ReadFile(file, o) },
	}
	for name, parse := range boundaries {
		t.Run(name, func(t *testing.T) {
			exact := codemeta.ParseOptions{MaxBytes: int64(len(input)), MaxDepth: 2, MaxNodes: 3, MaxStringBytes: 3}
			if _, err := parse(exact); err != nil {
				t.Fatal(err)
			}
			if _, err := parse(codemeta.ParseOptions{MaxBytes: math.MaxInt64 - 1}); err != nil {
				t.Fatalf("largest supported byte limit: %v", err)
			}
			_, err := parse(codemeta.ParseOptions{MaxBytes: math.MaxInt64})
			var detail *codemeta.Error
			if !errors.Is(err, codemeta.ErrOptions) || !errors.As(err, &detail) || !strings.Contains(detail.Message, "MaxBytes must be less than math.MaxInt64") {
				t.Fatalf("unsupported byte limit: %v", err)
			}
			for _, options := range []codemeta.ParseOptions{
				{MaxBytes: exact.MaxBytes - 1}, {MaxDepth: exact.MaxDepth - 1}, {MaxNodes: exact.MaxNodes - 1}, {MaxStringBytes: exact.MaxStringBytes - 1},
			} {
				if _, err := parse(options); !errors.Is(err, codemeta.ErrLimit) {
					t.Fatalf("options %+v: %v", options, err)
				}
			}
		})
	}
}
