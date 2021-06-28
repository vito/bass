package bass

import "fmt"

type String string

func (value String) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *String:
		*x = value
		return nil
	case *string:
		*x = string(value)
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value String) String() string {
	// TODO: account for differences in escape sequences
	return fmt.Sprintf("%q", string(value))
}

// Eval returns the value.
func (value String) Eval(*Env) (Value, error) {
	return value, nil
}
