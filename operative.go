package bass

import "fmt"

type Operative struct {
	Formals Value
	Eformal Value
	Body    Value

	Env *Env
}

var _ Value = (*Operative)(nil)

var _ Combiner = (*Operative)(nil)

func (value *Operative) String() string {
	return "<op>"
}

func (value *Operative) Decode(dest interface{}) error {
	// TODO: assign to *Operative?
	return fmt.Errorf("Operative.Decode is not implemented")
}

func (value *Operative) Eval(*Env) (Value, error) {
	// TODO: test
	return value, nil
}

func (value *Operative) Call(val Value, env *Env) (Value, error) {
	sub := NewEnv(value.Env)

	err := sub.Define(value.Formals, val)
	if err != nil {
		return nil, err
	}

	err = sub.Define(value.Eformal, env)
	if err != nil {
		return nil, err
	}

	return value.Body.Eval(sub)
}
