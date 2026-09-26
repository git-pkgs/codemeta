package codemeta

import (
	"errors"
	"fmt"
)

var (
	ErrSyntax      = errors.New("invalid JSON")
	ErrUnsupported = errors.New("unsupported syntax")
	ErrType        = errors.New("unrepresentable document")
	ErrLimit       = errors.New("resource limit exceeded")
	ErrOptions     = errors.New("invalid parse options")
	ErrIO          = errors.New("input/output failure")
)

// Position identifies a one-based Unicode character line and column.
// Zero means no source position is available.
type Position struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
	Position
}

// Error retains a machine-readable category and the original I/O error, if any.
type Error struct {
	Diagnostic
	Category error
	Cause    error
}

func (e *Error) Error() string {
	if e.Line != 0 {
		return fmt.Sprintf("codemeta: %d:%d: %s", e.Line, e.Column, e.Message)
	}
	return "codemeta: " + e.Message
}
func (e *Error) Is(target error) bool { return target == e.Category }
func (e *Error) Unwrap() error        { return e.Cause }

func failure(category error, code, message string, pos Position) error {
	return &Error{Diagnostic: Diagnostic{Code: code, Message: message, Position: pos}, Category: category}
}
func ioFailure(err error) error {
	return &Error{Diagnostic: Diagnostic{Code: "io", Message: err.Error()}, Category: ErrIO, Cause: err}
}
