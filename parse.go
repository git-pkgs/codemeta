package codemeta

import (
	"io"
	"math"
	"os"
)

// ParseOptions bounds untrusted input. Zero selects the documented defaults.
// MaxBytes must be less than math.MaxInt64 to allow one extra byte for limit detection.
type ParseOptions struct {
	MaxBytes       int64
	MaxDepth       int
	MaxNodes       int
	MaxStringBytes int
	MaxDiagnostics int
}

const (
	defaultMaxBytes       = 1_048_576
	defaultMaxStringBytes = 4_194_304
)

func (o ParseOptions) defaults() (ParseOptions, error) {
	if o.MaxBytes < 0 || o.MaxDepth < 0 || o.MaxNodes < 0 || o.MaxStringBytes < 0 || o.MaxDiagnostics < 0 {
		return o, failure(ErrOptions, "options", "limits must not be negative; zero selects defaults", Position{})
	}
	if o.MaxBytes == math.MaxInt64 {
		return o, failure(ErrOptions, "options", "MaxBytes must be less than math.MaxInt64 to allow one extra byte for limit detection", Position{})
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = defaultMaxBytes
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 64
	}
	if o.MaxNodes == 0 {
		o.MaxNodes = 100000
	}
	if o.MaxStringBytes == 0 {
		o.MaxStringBytes = defaultMaxStringBytes
	}
	if o.MaxDiagnostics == 0 {
		o.MaxDiagnostics = 100
	}
	return o, nil
}

func Parse(data []byte) (*Document, error) { return ParseWithOptions(data, ParseOptions{}) }

func ParseWithOptions(data []byte, opts ParseOptions) (*Document, error) {
	opts, err := opts.defaults()
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > opts.MaxBytes {
		return nil, failure(ErrLimit, "byte_limit", "input exceeds byte limit", Position{1, 1})
	}
	s := scanner{data: data, opts: opts, line: 1, column: 1}
	root, err := s.document()
	if err != nil {
		return nil, err
	}
	if root.kind != Object {
		return nil, failure(ErrType, "root_type", "document root must be an object", root.pos)
	}
	return &Document{root: root, maxDiagnostics: opts.MaxDiagnostics, context: inspectContext(root.Get("@context"))}, nil
}

// Read leaves r open and consumes at most MaxBytes+1 bytes.
func Read(r io.Reader, opts ParseOptions) (*Document, error) {
	opts, err := opts.defaults()
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, failure(ErrOptions, "reader", "reader is nil", Position{})
	}
	data, err := io.ReadAll(io.LimitReader(r, opts.MaxBytes+1))
	if err != nil {
		return nil, ioFailure(err)
	}
	return ParseWithOptions(data, opts)
}

// ReadFile owns and closes the opened file.
func ReadFile(path string, opts ParseOptions) (*Document, error) {
	opts, err := opts.defaults()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, ioFailure(err)
	}
	doc, readErr := Read(f, opts)
	closeErr := f.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, ioFailure(closeErr)
	}
	return doc, nil
}
