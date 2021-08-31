package bass

import (
	"context"
	"runtime"
	"strings"
)

// Bindings maps Symbols to Values in a scope.
type Bindings map[Symbol]Value

// Docs maps Symbols to their documentation in a scope.
type Docs map[Symbol]Annotated

// Bindable is any value which may be used to destructure a value into bindings
// in a scope.
type Bindable interface {
	Value
	Bind(*Scope, Value) error
}

func BindConst(a, b Value) error {
	if !a.Equal(b) {
		return BindMismatchError{
			Need: a,
			Have: b,
		}
	}

	return nil
}

// Scope contains bindings from symbols to values, and parent scopes to
// delegate to during symbol lookup.
type Scope struct {
	Parents  []*Scope
	Bindings Bindings

	Commentary []Annotated
	Docs       Docs
}

// Assert that Scope is a Value.
var _ Value = (*Scope)(nil)

// NewScope constructs a Scope with empty bindings and the given parents.
func NewScope(ps ...*Scope) *Scope {
	return &Scope{
		Bindings: Bindings{},
		Docs:     Docs{},

		// XXX(hack): allocate a slice to prevent comparing w/ nil in tests
		Parents: append([]*Scope{}, ps...),
	}
}

func (value *Scope) String() string {
	return "<scope>"
}

func (value *Scope) Equal(other Value) bool {
	var o *Scope
	return other.Decode(&o) == nil && value == o
}

func (value *Scope) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Scope:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *Scope) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

// Eval returns the value.
func (value *Scope) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// Set assigns the value in the local bindings.
func (scope *Scope) Set(binding Symbol, value Value, docs ...string) {
	scope.Bindings[binding] = value

	if len(docs) > 0 {
		doc := scope.doc(binding, docs...)
		scope.Commentary = append(scope.Commentary, doc)
		scope.Docs[binding] = doc
	}
}

// Comment records commentary associated to the given value.
func (scope *Scope) Comment(val Value, docs ...string) {
	scope.Commentary = append(scope.Commentary, scope.doc(val, docs...))
}

// Get fetches the given binding.
//
// If a value is set in the local bindings, it is returned.
//
// If not, the parent scopes are queried in order.
//
// If no value is found, false is returned.
func (scope *Scope) Get(binding Symbol) (Value, bool) {
	val, found := scope.Bindings[binding]
	if found {
		return val, found
	}

	for _, parent := range scope.Parents {
		val, found = parent.Get(binding)
		if found {
			return val, found
		}
	}

	return nil, false
}

// Doc fetches the given binding's documentation.
//
// If a value is set in the local bindings, its documentation is returned.
//
// If not, the parent scopes are queried in order.
//
// If no value is found, false is returned.
func (scope *Scope) GetWithDoc(binding Symbol) (Annotated, bool) {
	value, found := scope.Bindings[binding]
	if found {
		annotated, found := scope.Docs[binding]
		if found {
			annotated.Value = value
			return annotated, true
		}

		return Annotated{
			Value: value,
		}, true
	}

	for _, parent := range scope.Parents {
		annotated, found := parent.GetWithDoc(binding)
		if found {
			return annotated, true
		}
	}

	return Annotated{}, false
}

func (scope *Scope) doc(val Value, docs ...string) Annotated {
	doc := Annotated{
		Value:   val,
		Comment: strings.Join(docs, "\n\n"),
	}

	_, file, line, ok := runtime.Caller(2)
	if ok {
		doc.Range.Start.File = file
		doc.Range.Start.Ln = line
		doc.Range.End.File = file
		doc.Range.End.Ln = line
	}

	return doc
}
