package bass

import (
	"fmt"
	"sort"
)

type Object map[Keyword]Value

var _ Value = Object(nil)

type kv struct {
	k Keyword
	v Value
}

type kvs []kv

func (kvs kvs) Len() int           { return len(kvs) }
func (kvs kvs) Less(i, j int) bool { return kvs[i].k < kvs[j].k }
func (kvs kvs) Swap(i, j int)      { kvs[i], kvs[j] = kvs[j], kvs[i] }

func (value Object) String() string {
	out := "{"

	kvs := make(kvs, 0, len(value))
	for k, v := range value {
		kvs = append(kvs, kv{k, v})
	}
	sort.Sort(kvs)

	l := len(kvs)
	for i, kv := range kvs {
		out += fmt.Sprintf("%s %s", kv.k, kv.v)

		if i+1 < l {
			out += " "
		}
	}

	out += "}"

	return out
}

func (value Object) Equal(other Value) bool {
	var o Object
	if err := other.Decode(&o); err != nil {
		return false
	}

	if len(value) != len(o) {
		return false
	}

	for k, v := range value {
		ov, found := o[k]
		if !found {
			return false
		}

		if !ov.Equal(v) {
			return false
		}
	}

	return true
}

func (value Object) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Object:
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
func (value Object) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}
