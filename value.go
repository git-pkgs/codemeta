// Package codemeta reads and validates CodeMeta metadata offline.
package codemeta

// Kind distinguishes JSON values. A zero Value is Missing.
type Kind uint8

const (
	Missing Kind = iota
	Null
	String
	Number
	Boolean
	Array
	Object
)

// Value owns its text and preserves source order and numeric spelling.
type Value struct {
	kind   Kind
	text   string
	items  []Value
	fields []Field
	pos    Position
}

type Field struct {
	Name     string
	Value    Value
	Position Position
}

func (v Value) Kind() Kind         { return v.kind }
func (v Value) Text() string       { return v.text }
func (v Value) Position() Position { return v.pos }
func (v Value) Items() []Value     { return append([]Value(nil), v.items...) }
func (v Value) Fields() []Field    { return append([]Field(nil), v.fields...) }

// Values returns array items, a single scalar or object, or nil for Missing.
// An explicit null remains a single Null value.
func (v Value) Values() []Value {
	if v.kind == Missing {
		return nil
	}
	if v.kind == Array {
		return v.Items()
	}
	return []Value{v}
}

// Get looks up the exact written key, without context resolution.
func (v Value) Get(name string) Value {
	for _, field := range v.fields {
		if field.Name == name {
			return field.Value
		}
	}
	return Value{}
}

type Document struct {
	root           Value
	maxDiagnostics int
	context        contextState
}

func (d *Document) Get(name string) Value {
	if d == nil {
		return Value{}
	}
	if d.context.usable() {
		return resolvedGet(d.root, name, contextTerms[d.context.version])
	}
	return d.root.Get(name)
}
func (d *Document) Fields() []Field {
	if d == nil {
		return nil
	}
	return d.root.Fields()
}
