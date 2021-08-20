package bass

import (
	"context"
	"strings"
)

// Bindings maps Symbols to Values in an environment.
type Bindings map[Symbol]Value

// Docs maps Symbols to their documentation in an environment.
type Docs map[Symbol]Annotated

// Bindable is any value which may be used to destructure a value into bindings
// in an environment.
type Bindable interface {
	Value
	Bind(*Env, Value) error
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

// Env contains bindings from symbols to values, and parent environments to
// delegate to during symbol lookup.
type Env struct {
	Bindings   Bindings
	Docs       Docs
	Commentary []Annotated
	Parents    []*Env
}

// Assert that Env is a Value.
var _ Value = (*Env)(nil)

// NewEnv constructs an Env with empty bindings and the given parents.
func NewEnv(ps ...*Env) *Env {
	return &Env{
		Bindings: Bindings{},
		Docs:     Docs{},

		// XXX(hack): allocate a slice to prevent comparing w/ nil in tests
		Parents: append([]*Env{}, ps...),
	}
}

func (value *Env) String() string {
	return "<env>"
}

func (value *Env) Equal(other Value) bool {
	var o *Env
	return other.Decode(&o) == nil && value == o
}

func (value *Env) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Env:
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

func (value *Env) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

// Eval returns the value.
func (value *Env) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// Set assigns the value in the local bindings.
func (env *Env) Set(binding Symbol, value Value, docs ...string) {
	env.Bindings[binding] = value

	if len(docs) > 0 {
		env.Docs[binding] = Annotated{
			Value:   value,
			Comment: strings.Join(docs, "\n\n"),
		}
	}
}

// Get fetches the given binding.
//
// If a value is set in the local bindings, it is returned.
//
// If not, the parent environments are queried in order.
//
// If no value is found, false is returned.
func (env *Env) Get(binding Symbol) (Value, bool) {
	val, found := env.Bindings[binding]
	if found {
		return val, found
	}

	for _, parent := range env.Parents {
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
// If not, the parent environments are queried in order.
//
// If no value is found, false is returned.
func (env *Env) GetWithDoc(binding Symbol) (Annotated, bool) {
	value, found := env.Bindings[binding]
	if found {
		annotated, found := env.Docs[binding]
		if found {
			annotated.Value = value
			return annotated, true
		}

		return Annotated{
			Value: value,
		}, true
	}

	for _, parent := range env.Parents {
		annotated, found := parent.GetWithDoc(binding)
		if found {
			return annotated, true
		}
	}

	return Annotated{}, false
}
