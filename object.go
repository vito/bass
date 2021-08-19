package bass

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
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
	case *Value:
		*x = value
		return nil
	case Decodable:
		return x.FromValue(value)
	case Value:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	default:
		return decodeStruct(value, dest)
	}
}

func hyphenate(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

func unhyphenate(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func (value Object) MarshalJSON() ([]byte, error) {
	m := map[string]Value{}

	for k, v := range value {
		m[unhyphenate(string(k))] = v
	}

	return MarshalJSON(m)
}

func (value *Object) UnmarshalJSON(payload []byte) error {
	var x interface{}
	err := UnmarshalJSON(payload, &x)
	if err != nil {
		return err
	}

	val, err := ValueOf(x)
	if err != nil {
		return err
	}

	obj, ok := val.(Object)
	if !ok {
		return fmt.Errorf("expected Object from ValueOf, got %T", val)
	}

	*value = obj

	return nil
}

// Eval returns the value.
func (value Object) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (object Object) Clone() Object {
	clone := make(Object, len(object))

	for k, v := range object {
		clone[k] = v
	}

	return clone
}

func isOptional(segs []string) bool {
	for _, seg := range segs {
		if seg == "omitempty" {
			return true
		}
	}
	return false
}

func decodeStruct(value Object, dest interface{}) error {
	rt := reflect.TypeOf(dest)
	if rt.Kind() != reflect.Ptr {
		return fmt.Errorf("decode into non-pointer %T", dest)
	}

	re := rt.Elem()
	rv := reflect.ValueOf(dest).Elem()

	if re.Kind() != reflect.Struct {
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}

	for i := 0; i < re.NumField(); i++ {
		field := re.Field(i)

		tag := field.Tag.Get("json")
		segs := strings.Split(tag, ",")
		name := segs[0]
		if name == "" {
			continue
		}

		key := KeywordFromJSONKey(name)

		var found bool
		val, found := value[key]
		if !found {
			if isOptional(segs) {
				continue
			}

			return fmt.Errorf("missing key %s", key)
		}

		if rv.Field(i).Kind() == reflect.Ptr {
			x := reflect.New(field.Type.Elem())

			err := val.Decode(x.Interface())
			if err != nil {
				return fmt.Errorf("decode (%T).%s: %w", dest, field.Name, err)
			}

			rv.Field(i).Set(x)
		} else {
			err := val.Decode(rv.Field(i).Addr().Interface())
			if err != nil {
				return fmt.Errorf("decode (%T).%s: %w", dest, field.Name, err)
			}
		}
	}

	return nil
}
