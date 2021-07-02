package bass

import (
	"strings"
)

// Bindings maps Symbols to Values in an environment.
type Bindings map[Symbol]Value

// Docs maps Symbols to their documentation in an environment.
type Docs map[Symbol]string

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
	}

	return DecodeError{
		Source:      value,
		Destination: dest,
	}
}

// Eval returns the value.
func (value *Env) Eval(env *Env, cont Cont) (ReadyCont, error) {
	return cont.Call(value), nil
}

// Set assigns the value in the local bindings.
func (env *Env) Set(binding Symbol, value Value, docs ...string) {
	env.Bindings[binding] = value

	if len(docs) > 0 {
		env.Docs[binding] = strings.Join(docs, "\n\n")
	}
}

// Define destructures value as binding.
func (env *Env) Define(binding Value, value Value) error {
	var i Ignore
	if err := binding.Decode(&i); err == nil {
		return nil
	}

	var s Symbol
	if err := binding.Decode(&s); err == nil {
		env.Set(s, value)
		return nil
	}

	var e Empty
	if err := binding.Decode(&e); err == nil {
		if err := value.Decode(&e); err == nil {
			return nil
		} else {
			return BindMismatchError{
				Need: binding,
				Have: value,
			}
		}
	}

	var b List
	if err := binding.Decode(&b); err == nil {
		if err := value.Decode(&e); err == nil {
			// empty value given for list
			return BindMismatchError{
				Need: binding,
				Have: value,
			}
		}

		var v List
		if err := value.Decode(&v); err == nil {
			err := env.Define(b.First(), v.First())
			if err != nil {
				return err
			}

			err = env.Define(b.Rest(), v.Rest())
			if err != nil {
				return err
			}

			return nil
		} else {
			return BindMismatchError{
				Need: binding,
				Have: value,
			}
		}
	}

	return CannotBindError{binding}
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
func (env *Env) GetWithDoc(binding Symbol) (Value, string, bool) {
	val, found := env.Bindings[binding]
	if found {
		return val, env.Docs[binding], true
	}

	for _, parent := range env.Parents {
		val, doc, found := parent.GetWithDoc(binding)
		if found {
			return val, doc, true
		}
	}

	return nil, "", false
}
