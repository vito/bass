package bass

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

func IsList(val Value) bool {
	switch x := val.(type) {
	case Empty:
		return true
	case List:
		return IsList(x.Rest())
	default:
		return false
	}
}
