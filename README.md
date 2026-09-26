# codemeta

A Go library for parsing `codemeta.json` files and validating software metadata, with no third-party dependencies. Reads bytes, readers, or files and preserves source positions, unknown fields, and numeric spelling. The same source builds with Go and TinyGo.

## Installation

```bash
go get github.com/git-pkgs/codemeta
```

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/git-pkgs/codemeta"
)

func main() {
	doc, err := codemeta.ReadFile("codemeta.json", codemeta.ParseOptions{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Name: %s\n", doc.Name())
	fmt.Printf("CodeMeta context: %s\n", doc.Version())
	fmt.Printf("Software version: %s\n", doc.SoftwareVersion().Text())
	for _, author := range doc.Author() {
		fmt.Println(author.Name(), author.GivenName(), author.FamilyName())
	}
	for _, issue := range doc.Validate() {
		fmt.Println(issue.Line, issue.Column, issue.Path, issue.Code, issue.Message)
	}
}
```

Use `Parse(data)` for bytes from a Git blob or another source, `ParseWithOptions(data, opts)` to set limits, or `Read(reader, opts)` for an `io.Reader`. `Read` leaves the reader open. Repository discovery, paths, blob identity, and concurrency limits remain with the caller.

Documents are immutable and can be read concurrently. Parsing copies retained text, so callers may reuse input buffers after a call returns. Collection accessors return copies of their slices.

### Metadata

`Name` and `Description` return text. `CodeRepository`, `SoftwareVersion`, `License`, `Keywords`, `ProgrammingLanguages`, `DatePublished`, `DateModified`, `DevelopmentStatus`, and `Identifier` return values that retain the written form. `Version` identifies the pinned CodeMeta context. It does not return the software version.

Use `Get` to access any field, including unknown fields and invalid metadata:

```go
fmt.Println(doc.Get("issueTracker").Text())
for _, keyword := range doc.Keywords().Values() {
	fmt.Println(keyword.Text())
}
```

Values expose `Kind`, `Text`, `Items`, `Fields`, and `Position`. `Items` returns only array elements; `Values` returns array elements or a single scalar or object. A missing field has kind `Missing` and no values, while explicit JSON `Null` remains a value. Numbers retain their spelling and precision. Dates retain their text.

`Author`, `Contributor`, `Maintainer`, `CopyrightHolder`, and `Funder` return ordered agent views, accepting either a single value or an array. `Agent.Kind` distinguishes people, organisations, strings, ID references, roles, and conflicting fields. `Value` preserves each original value; `Get` accesses its fields. `RoleName` and `Agents` expose a role and its nested agents without discarding the wrapper.

`Document.Get` and `Agent.Get` resolve known aliases and prefixed or full term IRIs when the declared context is usable. An exact written key takes precedence. `Value.Get` always uses the exact key. `Fields` retains written keys and their positions, including both keys when aliases conflict. Both `http://schema.org/` and `https://schema.org/` resolve to the same schema term. CodeMeta namespace IRIs remain specific to each pinned context.

### Errors

The library returns errors without logging or terminating the process. Use `errors.Is` with `ErrSyntax`, `ErrType`, `ErrUnsupported`, `ErrLimit`, `ErrOptions`, or `ErrIO` to distinguish failures. `errors.As` exposes a `*codemeta.Error` with a diagnostic. I/O errors retain their original cause, including `fs.ErrNotExist`.

`Validate` reports independent diagnostics in source order, with a field-path tie-breaker. Positions use one-based lines and Unicode character columns. Diagnostic codes and paths are intended for programmatic use; messages explain the problem.

## Compatibility

Parsing and validation are separate. Parsing requires valid UTF-8 JSON with an object root. Duplicate keys, comments, trailing commas, non-finite numbers, and lone surrogate escapes are errors. A UTF-8 BOM produces `ErrUnsupported`; escaped NUL is preserved, while a raw control character is a syntax error. A successful parse alone does not establish CodeMeta validity.

Validation uses checked-in CodeMeta 2.0, 3.0, and master contexts, plus type expectations generated from upstream `properties_description.csv`. [Source revisions and checksums](internal/contexts/sources.json) pin those inputs. The master URL always refers to the checked-in snapshot; it never changes with the date or a network response. The DOI URL for 2.0, published w3id URLs, raw context URLs, HTTP spellings, and trailing slashes are recognised.

Missing contexts produce `missing_context`; unknown contexts produce `unsupported_version`. Inline contexts equivalent to a pinned context are recognised. Other inline definitions produce `context_modified`, and combinations of different CodeMeta versions produce `context_conflict`. Unsupported or modified contexts disable term and range checks in their scope, while preserving all input. Nested contexts apply to their own objects. `Version` can still identify a recognised context in an array that also contains unsupported entries; inspect diagnostics before using that as a validity signal.

Context arrays are inspected in order. A partial inline definition such as `{"name":"schema:name"}` is accepted after a recognised context if it leaves the pinned definition unchanged. Placing it before that context produces `context_modified`; later entries do not retroactively resolve it. The detected `Version` is retained, but resolved accessors and term checks are disabled for that scope.

Terms found only in another pinned context produce `term_version`, while undefined terms produce `unknown_term`. Validation checks JSON value shapes, URL syntax, and calendar dates, without requiring any metadata properties. Date checks accept year, year-month, or calendar-date forms; date-time ranges also accept RFC 3339. URL checks do not resolve repositories, register ORCIDs, or check SPDX identifiers against a current list.

Validation is limited to pinned-context term resolution and the checks described above. JSON-LD expansion and compaction are outside its scope, so a clean result does not establish JSON-LD conformance. Writing, crosswalks, citation formatting, repository scanning, and network resolution are also outside this library's scope.

## Limits

Default limits are 1 MiB of input, depth 64, 100,000 nodes including object keys, 4 MiB of decoded string content and numeric text, and 100 validation diagnostics. The root is depth one. `ParseOptions` allows larger limits; zero selects the defaults and negative limits are errors. `MaxBytes` must be less than `math.MaxInt64`, leaving room to read one extra byte for limit detection; `math.MaxInt64` returns `ErrOptions`. When diagnostics are omitted, the last entry is `diagnostic_limit`, within the requested cap. A blocked reader requires a deadline supplied by its caller.

## Testing

```bash
go test ./...
go test -race ./...
go test -run '^$' -fuzz=FuzzParse -fuzztime=30s
go test -run '^$' -fuzz=FuzzRead -fuzztime=30s
CODEMETA_CORPUS=all go test -run TestCorpus ./...
go test -run '^$' -bench=. -benchmem
```

Tests use checked-in fixtures and run offline. The compact corpus runs by default; `CODEMETA_CORPUS=all` includes the larger compatibility sample and its benchmarks. Manifests record original bytes' hashes, source occurrences, parse outcomes, context versions, and diagnostics. [Generator comparisons](testdata/comparisons/differences.json) record tested differences and their reasons.

`go generate ./...` regenerates validation tables from the pinned contexts and property descriptions. Generation rejects unsupported context constructs and unknown type expressions. CI checks generated output, the dependency graph, race tests, fuzz targets, and a TinyGo consumer build without build tags or fallback implementations.

Run the example with `go run ./examples/read path/to/codemeta.json`, or supply `-` to read a Git blob through standard input. The history integration test adds, edits, moves, deletes, and reintroduces metadata in a temporary Git repository, and checks continuation after a malformed blob.

## License

[MIT](LICENSE). Imported fixtures and context documents retain their upstream attribution and licenses.
