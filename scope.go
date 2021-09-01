package bass

import (
	"context"
	"runtime"
	"sort"
	"strings"
)

// Scope contains bindings from symbols to values, and parent scopes to
// delegate to during symbol lookup.
type Scope struct {
	Parents  []*Scope
	Bindings Bindings

	Commentary []Annotated
	Docs       Docs

	printing bool
}

// Bindings maps Keywords to Values in a scope.
type Bindings map[Keyword]Value

// NewScope constructs a new *Scope with initial bindings.
func (bindings Bindings) Scope(parents ...*Scope) *Scope {
	return NewScope(bindings, parents...)
}

// Docs maps Symbols to their documentation in a scope.
type Docs map[Keyword]Annotated

// NewEmptyScope constructs a new scope with no bindings and
// optional parents.
func NewEmptyScope(parents ...*Scope) *Scope {
	return &Scope{
		Parents:  parents,
		Bindings: Bindings{},
		Docs:     Docs{},
	}
}

// NewScope constructs a new scope with the given bindings and
// optional parents.
func NewScope(bindings Bindings, parents ...*Scope) *Scope {
	return &Scope{
		Parents:  parents,
		Bindings: bindings,
		Docs:     Docs{},
	}
}

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

// Assert that Scope is a Value.
var _ Value = (*Scope)(nil)

func (value *Scope) String() string {
	if value.isPrinting() {
		return "{...}" // handle recursion or general noisiness
	}

	value.startPrinting()
	defer value.finishPrinting()

	bind := []Value{}

	kvs := make(kvs, 0, len(value.Bindings))
	for k, v := range value.Bindings {
		kvs = append(kvs, kv{Keyword(k), v})
	}

	sort.Sort(kvs)

	for _, kv := range kvs {
		bind = append(bind, kv.k, kv.v)
	}

	for _, parent := range value.Parents {
		bind = append(bind, parent)
	}

	return formatList(NewList(bind...), "{", "}")
}

func (value *Scope) isPrinting() bool {
	return value.printing
}

func (value *Scope) startPrinting() {
	value.printing = true
}

func (value *Scope) finishPrinting() {
	value.printing = false
}

func (value *Scope) Equal(o Value) bool {
	var other *Scope
	if err := o.Decode(&other); err != nil {
		return false
	}

	if other == value {
		return true
	}

	if len(other.Parents) != len(value.Parents) {
		return false
	}

	for i, p := range value.Parents {
		if !p.Equal(other.Parents[i]) {
			return false
		}
	}

	if len(other.Bindings) != len(value.Bindings) {
		return false
	}

	for k, v := range value.Bindings {
		ov, found := other.Bindings[k]
		if !found {
			return false
		}

		if !v.Equal(ov) {
			return false
		}
	}

	return true
}

func (value *Scope) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Scope:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Object:
		o := Object{}

		for _, parent := range value.Parents {
			var p Object
			err := parent.Decode(&o)
			if err != nil {
				return err
			}

			for k, v := range p {
				o[k] = v
			}
		}

		for k, v := range value.Bindings {
			o[k] = v
		}

		*x = o
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
func (scope *Scope) Set(binding Keyword, value Value, docs ...string) {
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
func (scope *Scope) Get(binding Keyword) (Value, bool) {
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
func (scope *Scope) GetWithDoc(binding Keyword) (Annotated, bool) {
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
