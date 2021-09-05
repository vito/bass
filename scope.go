package bass

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"

	"github.com/brentp/intintmap"
)

// Scope contains bindings from symbols to values, and parent scopes to
// delegate to during symbol lookup.
type Scope struct {
	// an optional name for the scope, used to prettify .String on 'standard'
	// environments
	Name string

	Parents  []*Scope
	Slots    []*Slot
	Bindings *intintmap.Map

	Commentary []Annotated
	docs       Docs

	printing bool
}

type Slot struct {
	Binding Symbol
	Value   Value
}

// Bindings maps Symbols to Values in a scope.
type Bindings map[string]Value

// NewScope constructs a new *Scope with initial bindings.
func (bindings Bindings) Scope(parents ...*Scope) *Scope {
	scope := NewEmptyScope(parents...)
	for k, v := range bindings {
		scope.Def(k, v)
	}

	return scope
}

// Docs maps Symbols to their documentation in a scope.
type Docs map[Symbol]Annotated

// NewEmptyScope constructs a new scope with no bindings and
// optional parents.
func NewEmptyScope(parents ...*Scope) *Scope {
	return &Scope{
		Parents:  parents,
		Bindings: intintmap.New(10, 0.8),
	}
}

// Bindable is any value which may be used to destructure a value into bindings
// in a scope.
type Bindable interface {
	Value
	Bind(*Scope, Value) error
}

func BindConst(a, b Value) error {
	if !a.Equal(b) {
		return BindMismatchError{
			Need: a,
			Have: b,
		}
	}

	return nil
}

// Assert that Scope is a Value.
var _ Value = (*Scope)(nil)

func (value *Scope) String() string {
	if value.Name != "" {
		return fmt.Sprintf("<scope: %s>", value.Name)
	}

	if value.isPrinting() {
		return "{...}" // handle recursion or general noisiness
	}

	value.startPrinting()
	defer value.finishPrinting()

	bind := []Value{}

	for _, slot := range value.Slots {
		bind = append(bind, slot.Binding, slot.Value)
	}

	for _, parent := range value.Parents {
		bind = append(bind, parent)
	}

	return formatList(NewList(bind...), "{", "}")
}

func (value *Scope) isPrinting() bool {
	return value.printing
}

func (value *Scope) startPrinting() {
	value.printing = true
}

func (value *Scope) finishPrinting() {
	value.printing = false
}

func (value *Scope) Equal(o Value) bool {
	// TODO: use Reduce instead to do a deep comparison of all bindings
	var other *Scope
	if err := o.Decode(&other); err != nil {
		return false
	}

	if other == value {
		return true
	}

	count := 0

	var errMismatch = errors.New("mismatch")
	err := value.Each(func(k Symbol, v Value) error {
		ov, found := other.Get(k)
		if !found || !v.Equal(ov) {
			// use an error to short-circuit
			return errMismatch
		}

		count++

		return nil
	})
	if err != nil {
		return false
	}

	otherCount := 0
	err = other.Each(func(Symbol, Value) error {
		otherCount++
		if otherCount > count {
			// has extra keys
			return errMismatch
		}

		// fewer keys should be impossible given we check if all of the
		// left-hand side values are bound

		return nil
	})
	return err == nil
}

func (value *Scope) IsEmpty() bool {
	empty := true
	_ = value.Each(func(Symbol, Value) error {
		empty = false
		return errors.New("im convinced")
	})

	return empty
}

func (value *Scope) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case **Scope:
		*x = value
		return nil
	case *Scope:
		*x = *value
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

func (value *Scope) Copy() *Scope {
	copied := NewEmptyScope()
	_ = value.Each(func(k Symbol, v Value) error {
		copied.Set(k, v)
		return nil
	})

	return copied
}

// Reduce calls f for each binding-value pair mapped by the scope.
//
// Note that shadowed bindings will be skipped.
func (value *Scope) Each(f func(Symbol, Value) error) error {
	return value.eachShadow(f, map[Symbol]bool{})
}

func (value *Scope) eachShadow(f func(Symbol, Value) error, called map[Symbol]bool) error {
	for _, slot := range value.Slots {
		if called[slot.Binding] {
			continue
		}

		called[slot.Binding] = true

		err := f(slot.Binding, slot.Value)
		if err != nil {
			return fmt.Errorf("%s: %w", slot.Binding, err)
		}
	}

	for _, p := range value.Parents {
		err := p.eachShadow(f, called)
		if err != nil {
			return err
		}
	}

	return nil
}

func hyphenate(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

func unhyphenate(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}

func (value *Scope) MarshalJSON() ([]byte, error) {
	m := map[string]Value{}

	_ = value.Each(func(k Symbol, v Value) error {
		m[unhyphenate(k.String())] = v
		return nil
	})

	return MarshalJSON(m)
}

func (value *Scope) UnmarshalJSON(payload []byte) error {
	var x interface{}
	err := UnmarshalJSON(payload, &x)
	if err != nil {
		return err
	}

	val, err := ValueOf(x)
	if err != nil {
		return err
	}

	scope, ok := val.(*Scope)
	if !ok {
		return fmt.Errorf("expected Object from ValueOf, got %T", val)
	}

	*value = *scope

	return nil
}

// Eval returns the value.
func (value *Scope) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// Set assigns the value in the local bindings.
func (scope *Scope) Set(binding Symbol, value Value, docs ...string) {
	slot, exists := scope.Bindings.Get(int64(binding))
	if exists {
		slot := scope.Slots[slot]
		slot.Value = value
		return
	}

	scope.Bindings.Put(int64(binding), int64(len(scope.Slots)))
	scope.Slots = append(scope.Slots, &Slot{
		Binding: binding,
		Value:   value,
	})

	if len(docs) > 0 {
		doc := annotate(binding, docs...)
		scope.Commentary = append(scope.Commentary, doc)

		scope.SetDoc(binding, doc)
	}
}

func (scope *Scope) GetDoc(binding Symbol) (Annotated, bool) {
	if scope.docs == nil {
		return Annotated{}, false
	}

	doc, found := scope.docs[binding]
	return doc, found
}

func (scope *Scope) SetDoc(binding Symbol, doc Annotated) {
	if scope.docs == nil {
		scope.docs = Docs{}
	}

	scope.docs[binding] = doc
}

// Comment records commentary associated to the given value.
func (scope *Scope) Comment(val Value, docs ...string) {
	scope.Commentary = append(scope.Commentary, annotate(val, docs...))
}

// Get fetches the given binding.
//
// If a value is set in the local bindings, it is returned.
//
// If not, the parent scopes are queried in order.
//
// If no value is found, false is returned.
func (scope *Scope) Get(binding Symbol) (Value, bool) {
	slot, found := scope.Bindings.Get(int64(binding))
	if found {
		return scope.Slots[slot].Value, found
	}

	for _, parent := range scope.Parents {
		val, found := parent.Get(binding)
		if found {
			return val, found
		}
	}

	return nil, false
}

// Doc fetches the given binding's documentation.
//
// If a value is set in the local bindings, its documentation is returned.
//
// If not, the parent scopes are queried in order.
//
// If no value is found, false is returned.
func (scope *Scope) GetWithDoc(binding Symbol) (Annotated, bool) {
	slot, found := scope.Bindings.Get(int64(binding))
	if found {
		value := scope.Slots[slot].Value

		annotated, found := scope.GetDoc(binding)
		if found {
			annotated.Value = value
			return annotated, true
		}

		return Annotated{
			Value: value,
		}, true
	}

	for _, parent := range scope.Parents {
		annotated, found := parent.GetWithDoc(binding)
		if found {
			return annotated, true
		}
	}

	return Annotated{}, false
}

func (scope *Scope) Def(name string, v Value, commentary ...string) {
	scope.Set(NewSymbol(name), v, commentary...)
}

func (scope *Scope) Defn(name, signature string, f interface{}, commentary ...string) {
	scope.Set(NewSymbol(name), Func(name, signature, f), commentary...)
}

func (scope *Scope) Defop(name, signature string, f interface{}, commentary ...string) {
	scope.Set(NewSymbol(name), Op(name, signature, f), commentary...)
}

func annotate(val Value, docs ...string) Annotated {
	annotated := Annotated{
		Value:   val,
		Comment: strings.Join(docs, "\n\n"),
	}

	_, file, line, ok := runtime.Caller(2)
	if ok {
		annotated.Range.Start.File = file
		annotated.Range.Start.Ln = line
		annotated.Range.End.File = file
		annotated.Range.End.Ln = line
	}

	return annotated
}

type kv struct {
	k Symbol
	v Value
}

type kvs []kv

func (kvs kvs) Len() int           { return len(kvs) }
func (kvs kvs) Less(i, j int) bool { return kvs[i].k < kvs[j].k }
func (kvs kvs) Swap(i, j int)      { kvs[i], kvs[j] = kvs[j], kvs[i] }

func isOptional(segs []string) bool {
	for _, seg := range segs {
		if seg == "omitempty" {
			return true
		}
	}
	return false
}

func decodeStruct(value *Scope, dest interface{}) error {
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

		sym := SymbolFromJSONKey(name)

		var found bool
		val, found := value.Get(sym)
		if !found {
			if isOptional(segs) {
				continue
			}

			return fmt.Errorf("missing key %s", sym)
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
