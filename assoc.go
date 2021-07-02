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

func (value Assoc) Eval(env *Env, cont Cont) ReadyCont {
	if len(value) == 0 {
		return cont.Call(Object{}, nil)
	}

	assoc := value[0]
	rest := value[1:]

	return rest.Eval(env, Continue(func(objRes Value) ReadyCont {
		return assoc.A.Eval(env, Continue(func(keyRes Value) ReadyCont {
			var obj Object
			err := objRes.Decode(&obj)
			if err != nil {
				return cont.Call(nil, err)
			}

			var key Keyword
			err = keyRes.Decode(&key)
			if err != nil {
				return cont.Call(nil, BadKeyError{
					Value: keyRes,
				})
			}

			return assoc.D.Eval(env, Continue(func(res Value) ReadyCont {
				obj[key] = res
				return cont.Call(obj, nil)
			}))
		}))
	}))
}
