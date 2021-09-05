package bass

import (
	"context"
	"fmt"
	"sync"
)

type Symbol int

const SymbolInvalid Symbol = 0

var _ Value = Symbol(0)

var symbols = []string{
	// corresponds to SymbolInvalid
	"",
}
var interned = map[string]int{}
var internL = new(sync.Mutex)

func NewSymbol(sym string) Symbol {
	internL.Lock()
	defer internL.Unlock()

	exists, found := interned[sym]
	if found {
		return Symbol(exists)
	}

	num := len(symbols)
	interned[sym] = num
	symbols = append(symbols, sym)
	return Symbol(num)
}

func SymbolFromJSONKey(key string) Symbol {
	return NewSymbol(hyphenate(key))
}

func (value Symbol) String() string {
	return symbols[value]
}

func (value Symbol) Keyword() Keyword {
	return Keyword(value)
}

func (value Symbol) JSONKey() string {
	return unhyphenate(value.String())
}

func (value Symbol) Equal(other Value) bool {
	var o Symbol
	return other.Decode(&o) == nil && value == o
}

func (value Symbol) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Symbol:
		*x = value
		return nil
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Bindable:
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
func (value Symbol) Eval(_ context.Context, scope *Scope, cont Cont) ReadyCont {
	res, found := scope.Get(value)
	if !found {
		return cont.Call(nil, UnboundError{value})
	}

	return cont.Call(res, nil)
}

var _ Bindable = Symbol(0)

func (binding Symbol) Bind(scope *Scope, val Value) error {
	scope.Set(binding, val)
	return nil
}

var _ Applicative = Symbol(0)

func (app Symbol) Unwrap() Combiner {
	return SymbolOperative{app}
}

var _ Combiner = Symbol(0)

func (combiner Symbol) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(SymbolOperative{combiner}).Call(ctx, val, scope, cont)
}

type SymbolOperative struct {
	Symbol Symbol
}

var _ Value = SymbolOperative{}

func (value SymbolOperative) String() string {
	return fmt.Sprintf("(unwrap %s)", value.Symbol)
}

func (value SymbolOperative) Equal(other Value) bool {
	var o SymbolOperative
	return other.Decode(&o) == nil && value == o
}

func (value SymbolOperative) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *SymbolOperative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value SymbolOperative) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (op SymbolOperative) Call(_ context.Context, val Value, _ *Scope, cont Cont) ReadyCont {
	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, fmt.Errorf("call keyword: %w", err))
	}

	src := list.First()

	var res Value
	var found bool

	var srcScope *Scope
	if err := src.Decode(&srcScope); err == nil {
		res, found = srcScope.Get(op.Symbol)
	}

	if found {
		return cont.Call(res, nil)
	}

	var rest List
	err = list.Rest().Decode(&rest)
	if err != nil {
		return cont.Call(nil, err)
	}

	var empty Empty
	err = rest.Decode(&empty)
	if err == nil {
		return cont.Call(Null{}, nil)
	}

	return cont.Call(rest.First(), nil)
}
