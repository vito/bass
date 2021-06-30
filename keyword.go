package bass

import (
	"fmt"
	"strings"
)

type Keyword string

var _ Value = Keyword("")

func hyphenate(value Keyword) string {
	return strings.ReplaceAll(string(value), "_", "-")
}

func (value Keyword) String() string {
	// TODO: test
	return fmt.Sprintf(":%s", hyphenate(value))
}

func (value Keyword) Equal(other Value) bool {
	var o Keyword
	return other.Decode(&o) == nil && value == o
}

func (value Keyword) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Keyword:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value Keyword) Eval(*Env) (Value, error) {
	return value, nil
}

var _ Combiner = Keyword("")

func (combiner Keyword) Call(val Value, env *Env) (Value, error) {
	var list List
	err := val.Decode(&list)
	if err != nil {
		return nil, fmt.Errorf("call applicative: %w", err)
	}

	res, err := list.First().Eval(env)
	if err != nil {
		return nil, err
	}

	var obj Object
	err = res.Decode(&obj)
	if err != nil {
		return nil, err
	}

	return obj[combiner], nil
}
