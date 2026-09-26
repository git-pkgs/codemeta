package codemeta

import (
	"bytes"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	utf8ContinuationMask = 0xc0
	utf8Continuation     = 0x80
	firstPrintable       = 0x20
	decimalBase          = 10
	hexadecimalBase      = 16
)

type scanner struct {
	data                 []byte
	opts                 ParseOptions
	offset, line, column int
	nodes, strings       int
}

func (s *scanner) position() Position { return Position{s.line, s.column} }
func (s *scanner) syntax(message string) error {
	return failure(ErrSyntax, "syntax", message, s.position())
}
func (s *scanner) peek() byte {
	if s.offset == len(s.data) {
		return 0
	}
	return s.data[s.offset]
}
func (s *scanner) advance() {
	c := s.data[s.offset]
	s.offset++
	switch {
	case c == '\r':
		s.line++
		s.column = 1
	case c == '\n':
		if s.offset < 2 || s.data[s.offset-2] != '\r' {
			s.line++
		}
		s.column = 1
	case c&utf8ContinuationMask != utf8Continuation:
		s.column++
	}
}
func (s *scanner) space() {
	for s.offset < len(s.data) {
		switch s.peek() {
		case ' ', '\t', '\r', '\n':
			s.advance()
		default:
			return
		}
	}
}
func (s *scanner) document() (Value, error) {
	if bytes.HasPrefix(s.data, []byte{0xef, 0xbb, 0xbf}) {
		return Value{}, failure(ErrUnsupported, "bom", "UTF-8 byte order mark is unsupported", s.position())
	}
	s.space()
	if s.offset == len(s.data) {
		return Value{}, failure(ErrSyntax, "empty_input", "input is empty", s.position())
	}
	v, err := s.value(1)
	if err != nil {
		return Value{}, err
	}
	s.space()
	if s.offset != len(s.data) {
		return Value{}, s.syntax("unexpected content after document")
	}
	return v, nil
}
func (s *scanner) node() error {
	if s.nodes >= s.opts.MaxNodes {
		return failure(ErrLimit, "node_limit", "input exceeds node limit", s.position())
	}
	s.nodes++
	return nil
}
func (s *scanner) value(depth int) (Value, error) {
	if depth > s.opts.MaxDepth {
		return Value{}, failure(ErrLimit, "depth_limit", "input exceeds depth limit", s.position())
	}
	if err := s.node(); err != nil {
		return Value{}, err
	}
	v := Value{pos: s.position()}
	switch c := s.peek(); {
	case c == '{':
		v.kind = Object
		fields, err := s.object(depth)
		v.fields = fields
		return v, err
	case c == '[':
		v.kind = Array
		items, err := s.array(depth)
		v.items = items
		return v, err
	case c == '"':
		v.kind = String
		text, err := s.stringValue()
		v.text = text
		return v, err
	case c == 't':
		return s.literal(v, "true", Boolean)
	case c == 'f':
		return s.literal(v, "false", Boolean)
	case c == 'n':
		return s.literal(v, "null", Null)
	case c == '-' || digit(c):
		v.kind = Number
		text, err := s.number()
		v.text = text
		return v, err
	default:
		return Value{}, s.syntax("expected a JSON value")
	}
}
func (s *scanner) literal(v Value, text string, kind Kind) (Value, error) {
	for i := range len(text) {
		if s.offset == len(s.data) || s.peek() != text[i] {
			return Value{}, s.syntax("invalid JSON literal")
		}
		s.advance()
	}
	v.kind = kind
	if kind != Null {
		v.text = text
	}
	return v, nil
}
func (s *scanner) object(depth int) ([]Field, error) {
	s.advance()
	s.space()
	var fields []Field
	seen := make(map[string]struct{})
	if s.peek() == '}' {
		s.advance()
		return fields, nil
	}
	for {
		if s.peek() != '"' {
			return nil, s.syntax("expected a quoted object key")
		}
		if err := s.node(); err != nil {
			return nil, err
		}
		pos := s.position()
		key, err := s.stringValue()
		if err != nil {
			return nil, err
		}
		if _, exists := seen[key]; exists {
			return nil, failure(ErrSyntax, "duplicate_key", "duplicate object key: "+key, pos)
		}
		seen[key] = struct{}{}
		s.space()
		if s.peek() != ':' {
			return nil, s.syntax("expected colon after object key")
		}
		s.advance()
		s.space()
		value, err := s.value(depth + 1)
		if err != nil {
			return nil, err
		}
		fields = append(fields, Field{Name: key, Value: value, Position: pos})
		more, err := s.separator('}')
		if err != nil {
			return nil, err
		}
		if !more {
			return fields, nil
		}
	}
}
func (s *scanner) array(depth int) ([]Value, error) {
	s.advance()
	s.space()
	var items []Value
	if s.peek() == ']' {
		s.advance()
		return items, nil
	}
	for {
		value, err := s.value(depth + 1)
		if err != nil {
			return nil, err
		}
		items = append(items, value)
		more, err := s.separator(']')
		if err != nil {
			return nil, err
		}
		if !more {
			return items, nil
		}
	}
}
func (s *scanner) separator(end byte) (bool, error) {
	s.space()
	if s.peek() == end {
		s.advance()
		return false, nil
	}
	if s.peek() != ',' {
		return false, s.syntax("expected comma or closing delimiter")
	}
	s.advance()
	s.space()
	return true, nil
}
func digit(c byte) bool { return c >= '0' && c <= '9' }
func (s *scanner) digits() bool {
	start := s.offset
	for digit(s.peek()) {
		s.advance()
	}
	return s.offset > start
}
func (s *scanner) number() (string, error) {
	start := s.offset
	if s.peek() == '-' {
		s.advance()
	}
	if s.peek() == '0' {
		s.advance()
	} else if !s.digits() {
		return "", s.syntax("expected number digit")
	}
	if s.peek() == '.' {
		s.advance()
		if !s.digits() {
			return "", s.syntax("expected fractional digit")
		}
	}
	if s.peek() == 'e' || s.peek() == 'E' {
		s.advance()
		if s.peek() == '+' || s.peek() == '-' {
			s.advance()
		}
		if !s.digits() {
			return "", s.syntax("expected exponent digit")
		}
	}
	if err := s.stringBytes(s.offset - start); err != nil {
		return "", err
	}
	return string(s.data[start:s.offset]), nil
}
func (s *scanner) stringBytes(n int) error {
	if n > s.opts.MaxStringBytes-s.strings {
		return failure(ErrLimit, "string_limit", "input exceeds string byte limit", s.position())
	}
	s.strings += n
	return nil
}
func (s *scanner) stringValue() (string, error) {
	s.advance()
	var out []byte
	for s.offset < len(s.data) {
		switch c := s.peek(); {
		case c == '"':
			s.advance()
			return string(out), nil
		case c == '\\':
			s.advance()
			r, err := s.escape()
			if err != nil {
				return "", err
			}
			if err := s.stringBytes(utf8.RuneLen(r)); err != nil {
				return "", err
			}
			out = utf8.AppendRune(out, r)
		case c < firstPrintable:
			return "", s.syntax("unescaped control character in string")
		default:
			r, n := utf8.DecodeRune(s.data[s.offset:])
			if r == utf8.RuneError && n == 1 {
				return "", s.syntax("invalid UTF-8 in string")
			}
			if err := s.stringBytes(n); err != nil {
				return "", err
			}
			out = append(out, s.data[s.offset:s.offset+n]...)
			for range n {
				s.advance()
			}
		}
	}
	return "", s.syntax("unterminated string")
}
func (s *scanner) escape() (rune, error) {
	if s.offset == len(s.data) {
		return 0, s.syntax("unterminated escape")
	}
	c := s.peek()
	s.advance()
	switch c {
	case '"', '\\', '/':
		return rune(c), nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		return s.unicodeEscape()
	default:
		return 0, s.syntax("invalid string escape")
	}
}
func (s *scanner) hexRune() (rune, error) {
	var r rune
	for range 4 {
		if s.offset == len(s.data) {
			return 0, s.syntax("incomplete Unicode escape")
		}
		c := s.peek()
		var n byte
		switch {
		case c >= '0' && c <= '9':
			n = c - '0'
		case c >= 'a' && c <= 'f':
			n = c - 'a' + decimalBase
		case c >= 'A' && c <= 'F':
			n = c - 'A' + decimalBase
		default:
			return 0, s.syntax("invalid Unicode escape")
		}
		r = r*hexadecimalBase + rune(n)
		s.advance()
	}
	return r, nil
}
func (s *scanner) unicodeEscape() (rune, error) {
	pos := s.position()
	r, err := s.hexRune()
	if err != nil {
		return 0, err
	}
	if !utf16.IsSurrogate(r) {
		return r, nil
	}
	if r >= 0xdc00 || !bytes.HasPrefix(s.data[s.offset:], []byte(`\u`)) {
		return 0, failure(ErrSyntax, "lone_surrogate", "Unicode surrogate requires a pair", pos)
	}
	s.advance()
	s.advance()
	low, err := s.hexRune()
	if err != nil {
		return 0, err
	}
	if low < 0xdc00 || low > 0xdfff {
		return 0, failure(ErrSyntax, "lone_surrogate", "invalid Unicode surrogate pair", pos)
	}
	return utf16.DecodeRune(r, low), nil
}
