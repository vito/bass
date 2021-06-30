package bass

import "fmt"

type Assoc []Pair

var _ Value = Assoc(nil)

func (value Assoc) String() string {
	out := "{"

	l := len(value)
	for i, pair := range value {
		out += fmt.Sprintf("%s %s", pair.A, pair.D)

		if i+1 < l {
			out += " "
		}
	}

	out += "}"

	return out
}

func (value Assoc) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Assoc:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value Assoc) Equal(other Value) bool {
	var o Assoc
	if err := other.Decode(&o); err != nil {
		return false
	}

	if len(value) != len(o) {
		return false
	}

	for _, a := range value {
		var matched bool
		for _, b := range o {
			if a.A.Equal(b.A) {
				matched = true

				if !a.D.Equal(b.D) {
					return false
				}
			}
		}

		if !matched {
			return false
		}
	}

	return true
}

func (value Assoc) Eval(env *Env) (Value, error) {
	obj := Object{}
	for _, assoc := range value {
		k, err := assoc.A.Eval(env)
		if err != nil {
			return nil, err
		}

		var key Keyword
		err = k.Decode(&key)
		if err != nil {
			return nil, BadKeyError{
				Value: k,
			}
		}

		obj[key], err = assoc.D.Eval(env)
		if err != nil {
			return nil, err
		}
	}

	return obj, nil
}
