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

func Each(list List, cb func(Value) error) error {
	for list != (Empty{}) {
		err := cb(list.First())
		if err != nil {
			return err
		}

		err = list.Rest().Decode(&list)
		if err != nil {
			// TODO: better error
			return err
		}
	}

	return nil
}

func IsList(val Value) bool {
	var empty Empty
	err := val.Decode(&empty)
	if err == nil {
		return true
	}

	var list List
	err = val.Decode(&list)
	if err != nil {
		return false
	}

	return IsList(list.Rest())
}
