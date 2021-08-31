package bass

import (
	"context"
	"fmt"
)

type Keyword string

var _ Value = Keyword("")

func KeywordFromJSONKey(key string) Keyword {
	return Keyword(hyphenate(key))
}

func (value Keyword) JSONKey() string {
	return unhyphenate(string(value))
}

func (value Keyword) String() string {
	return fmt.Sprintf(":%s", string(value))
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
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Bindable:
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

// Eval returns the value.
func (value Keyword) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = Keyword("")

func (app Keyword) Unwrap() Combiner {
	return KeywordOperative{app}
}

var _ Combiner = Keyword("")

func (combiner Keyword) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(KeywordOperative{combiner}).Call(ctx, val, scope, cont)
}

type KeywordOperative struct {
	Keyword Keyword
}

var _ Value = KeywordOperative{}

func (value KeywordOperative) String() string {
	return fmt.Sprintf("(unwrap %s)", value.Keyword)
}

func (value KeywordOperative) Equal(other Value) bool {
	var o KeywordOperative
	return other.Decode(&o) == nil && value == o
}

func (value KeywordOperative) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *KeywordOperative:
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

func (value KeywordOperative) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (op KeywordOperative) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, fmt.Errorf("call keyword: %w", err))
	}

	src := list.First()

	var res Value
	var found bool

	var srcObj Object
	var srcScope *Scope
	if err := src.Decode(&srcObj); err == nil {
		res, found = srcObj[op.Keyword]
	} else if err := src.Decode(&srcScope); err == nil {
		res, found = srcScope.Get(Symbol(op.Keyword))
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

var _ Bindable = Keyword("")

func (binding Keyword) Bind(scope *Scope, val Value) error {
	return BindConst(binding, val)
}
