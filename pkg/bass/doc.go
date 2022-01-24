package bass

import (
	"context"
	"fmt"
	"strings"

	"github.com/vito/bass/pkg/ioctx"
)

var separator = fmt.Sprintf("\x1b[90m%s\x1b[0m", strings.Repeat("-", 50))

func PrintDocs(ctx context.Context, cont Cont, scope *Scope, forms ...Value) ReadyCont {
	w := ioctx.StderrFromContext(ctx)

	if len(forms) == 0 {
		for _, sym := range scope.Order {
			forms = append(forms, sym)
		}
	}

	fmt.Fprintln(w, separator)

	form := forms[0]
	return form.Eval(ctx, scope, Continue(func(val Value) Value {
		PrintBindingDocs(ctx, scope, form, val)

		if len(forms) == 1 {
			return cont.Call(Null{}, nil)
		}

		return PrintDocs(ctx, cont, scope, forms[1:]...)
	}))
}

// Predicates returns a list of all builtin predicates which return true for
// the given value.
func Predicates(val Value) []Symbol {
	var preds []Symbol
	for _, pred := range primPreds {
		if pred.check(val) {
			preds = append(preds, pred.name)
		}
	}

	return preds
}

func PrintBindingDocs(ctx context.Context, scope *Scope, form, val Value) {
	w := ioctx.StderrFromContext(ctx)

	fmt.Fprintf(w, "\x1b[32m%s\x1b[0m", form)

	for _, pred := range Predicates(val) {
		fmt.Fprintf(w, " \x1b[33m%s\x1b[0m", pred)
	}

	fmt.Fprintln(w)

	var annotated Annotated
	var doc string
	if err := val.Decode(&annotated); err == nil {
		_ = annotated.Meta.GetDecode(DocMetaBinding, &doc)
	}

	var app Applicative
	if err := val.Decode(&app); err == nil {
		val = app.Unwrap()
	}

	var operative *Operative
	if err := val.Decode(&operative); err == nil {
		fmt.Fprintln(w, "args:", operative.Bindings)
	}

	var builtin *Builtin
	if err := val.Decode(&builtin); err == nil {
		fmt.Fprintln(w, "args:", builtin.Formals)
	}

	if doc != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, doc)
	}

	fmt.Fprintln(w)
}

func Details(val Value) string {
	var constructor Symbol = "op"

	var app Applicative
	if err := val.Decode(&app); err == nil {
		constructor = "fn"

		val = app.Unwrap()
	}

	var operative *Operative
	if err := val.Decode(&operative); err == nil {
		return NewList(constructor, operative.Bindings).String()
	}

	var builtin *Builtin
	if err := val.Decode(&builtin); err == nil {
		return NewList(constructor, builtin.Formals).String()
	}

	return val.String()
}
