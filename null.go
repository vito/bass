package bass

import (
	"reflect"
)

type Null struct{}

func (Null) String() string {
	return "null"
}

// Decode replaces the destination with its zero-value.
//
// The reason for this is to be compatible with YAML maps written like so:
//
//   foo: bar
//   baz:
//
// Above, the value of 'baz' is null, but some projects (e.g. Concourse) treat
// this equivalent to an empty value (e.g. an empty string). Thus, we want to
// ensure we actually decode as if a zero-value of the target type was present.
//
// In addition, explicitly zero-ing out the value instead of doing a no-op
// prevents a key from keeping a stale value from a previous Decode.
func (value Null) Decode(dest interface{}) error {
	val := reflect.ValueOf(dest)
	if val.Kind() != reflect.Ptr {
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}

	elem := val.Elem()
	elem.Set(reflect.Zero(elem.Type()))

	return nil
}

// Eval returns the value.
func (value Null) Eval(*Env) (Value, error) {
	return value, nil
}
