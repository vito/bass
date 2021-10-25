package bass

import (
	"context"
	"fmt"

	"github.com/spy16/slurp/reader"
)

type Annotate struct {
	// Value the inner form whose value will be annotated.
	Value

	// Range is the source code location of the inner form.
	Range Range

	// Comment is a literal comment ahead of or following the inner form.
	Comment string

	// Meta is an optional binding form that will be evaluated to attach metadata
	// to the inner form's value.
	Meta *Bind
}

func (value Annotate) String() string {
	if value.Meta != nil {
		return fmt.Sprintf("^%s %s", value.Meta, *value.Meta)
	} else {
		return value.Value.String()
	}
}

func (value Annotate) MetaBind() Bind {
	var bind Bind
	if value.Meta != nil {
		bind = *value.Meta
	}

	if value.Comment != "" {
		bind = append(bind,
			Keyword("doc"), String(value.Comment))
	}

	return bind
}

func (value Annotate) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	bind := value.MetaBind()

	next := cont
	if len(bind) > 0 {
		next = Continue(func(res Value) Value {
			if value.Comment != "" {
				var ign Ignore
				if err := res.Decode(&ign); err == nil {
					scope.Comment(value.Comment)
				}
			}

			return bind.Eval(ctx, scope, Continue(func(metaVal Value) Value {
				var meta *Scope
				if err := metaVal.Decode(&meta); err != nil {
					return cont.Call(nil, err)
				}

				var ann Annotated
				if err := res.Decode(&ann); err == nil {
					meta.Parents = append(meta.Parents, ann.Meta)
					ann.Meta = meta
				} else {
					ann.Value = res
					ann.Meta = meta
				}

				var bnd Bindable
				if err := res.Decode(&bnd); err == nil {
					bnd.EachBinding(func(s Symbol, _ Range) error {
						bmeta := NewEmptyScope(meta)

						value.Range.ToMeta(bmeta)

						val, found := scope.Get(s)
						if found {
							var ann Annotated
							if err := val.Decode(&ann); err == nil {
								ann.Meta = NewEmptyScope(ann.Meta, bmeta)
							} else {
								ann.Value = val
								ann.Meta = bmeta
							}

							scope.Set(s, ann)
						}

						return nil
					})
				}

				return cont.Call(ann, nil)
			}))
		})
	}

	return value.Value.Eval(ctx, scope, WithFrame(ctx, &value, next))
}

func (value Annotate) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Annotate:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Bindable:
		var inner Bindable
		if err := value.Value.Decode(&inner); err != nil {
			return err
		}

		*x = AnnotateBinding{
			Bindable: inner,
			Range:    value.Range,
			MetaBind: value.MetaBind(),
		}

		return nil
	default:
		return value.Value.Decode(dest)
	}
}

func (value Annotate) MarshalJSON() ([]byte, error) {
	return nil, EncodeError{value}
}

type Annotated struct {
	// Value is the inner value.
	Value

	// Meta contains metadata about the inner value.
	Meta *Scope
}

func (value Annotated) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Annotated:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return value.Value.Decode(dest)
	}
}

func (value Annotated) MarshalJSON() ([]byte, error) {
	return MarshalJSON(value.Value)
}

func WithMeta(val Value, metaVal Value) (Value, error) {
	var nul Null
	if err := metaVal.Decode(&nul); err == nil {
		return val, nil
	}

	var meta *Scope
	if err := metaVal.Decode(&meta); err != nil {
		return nil, err
	}

	return Annotated{
		Value: val,
		Meta:  meta,
	}, nil
}

type AnnotateBinding struct {
	Bindable
	Range    Range
	MetaBind Bind
}

var _ Bindable = AnnotateBinding{}

func (value AnnotateBinding) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *AnnotateBinding:
		*x = value
		return nil
	case *Bindable:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return value.Bindable.Decode(dest)
	}
}

func (binding AnnotateBinding) Bind(ctx context.Context, scope *Scope, cont Cont, val Value, doc ...Annotated) ReadyCont {
	if len(binding.MetaBind) == 0 {
		return binding.Bindable.Bind(ctx, scope, cont, val, doc...)
	}

	return binding.MetaBind.Eval(ctx, scope, Continue(func(metaVal Value) Value {
		var meta *Scope
		if err := metaVal.Decode(&meta); err != nil {
			return cont.Call(nil, err)
		}

		binding.Range.ToMeta(meta)

		return binding.Bindable.Bind(ctx, scope, cont, Annotated{
			Value: val,
			Meta:  meta,
		}, doc...)
	}))
}

func (binding AnnotateBinding) EachBinding(cb func(Symbol, Range) error) error {
	return binding.Bindable.EachBinding(func(s Symbol, r Range) error {
		if r == (Range{}) {
			return cb(s, binding.Range)
		} else {
			return cb(s, r)
		}
	})
}

type Range struct {
	Start, End reader.Position
}

func (inner Range) IsWithin(outer Range) bool {
	if inner.Start.Ln < outer.Start.Ln {
		return false
	}

	if inner.End.Ln > outer.End.Ln {
		return false
	}

	if inner.Start.Ln == outer.Start.Ln {
		if inner.Start.Col < outer.Start.Col {
			return false
		}
	}

	if inner.End.Ln == outer.End.Ln {
		if inner.End.Col > outer.End.Col {
			return false
		}
	}

	return true
}

func (r Range) String() string {
	return fmt.Sprintf("%s:%d:%d..%d:%d", r.Start.File, r.Start.Ln, r.Start.Col, r.End.Ln, r.End.Col)
}

func (r Range) ToMeta(meta *Scope) {
	meta.Set("file", String(r.Start.File))
	meta.Set("line", Int(r.Start.Ln))
	meta.Set("column", Int(r.Start.Col))
}
