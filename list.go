package bass

import (
	"fmt"
)

type List interface {
	Value

	First() Value
	Rest() Value
}

func NewList(vals ...Value) List {
	var list List = Empty{}
	for i := len(vals) - 1; i >= 0; i-- {
		list = Pair{
			A: vals[i],
			D: list,
		}
	}

	return list
}

type Pair struct {
	A Value
	D Value
}

func (value Pair) Decode(dest interface{}) error {
	// TODO: implement this someday - it's not used by anything yet
	return fmt.Errorf("unimplemented")
}

func (value Pair) First() Value {
	return value.A
}

func (value Pair) Rest() Value {
	return value.D
}

type Empty struct{}

func (value Empty) Decode(dest interface{}) error {
	// TODO: implement this someday - it's not used by anything yet
	return fmt.Errorf("unimplemented")
}

func (Empty) First() Value {
	return Empty{}
}

func (Empty) Rest() Value {
	return Empty{}
}
