package bass

import (
	"context"
	"fmt"
	"strings"

	"github.com/vito/bass/ioctx"
)

var separator = fmt.Sprintf("\x1b[90m%s\x1b[0m", strings.Repeat("-", 50))

func PrintDocs(ctx context.Context, scope *Scope, syms ...Symbol) {
	w := ioctx.StderrFromContext(ctx)

	if len(syms) == 0 {
		for _, comment := range scope.Commentary {
			fmt.Fprintln(w, comment)
			fmt.Fprintln(w)
		}

		return
	}

	for _, sym := range syms {
		fmt.Fprintln(w, separator)
		PrintBindingDocs(ctx, scope, sym)
	}
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

func PrintBindingDocs(ctx context.Context, scope *Scope, sym Symbol) {
	w := ioctx.StderrFromContext(ctx)

	fmt.Fprintf(w, "\x1b[32m%s\x1b[0m", sym)

	val, found := scope.Get(sym)
	if !found {
		fmt.Fprintf(w, " \x1b[31msymbol not bound\x1b[0m\n")
	} else {
		for _, pred := range Predicates(val) {
			fmt.Fprintf(w, " \x1b[33m%s\x1b[0m", pred)
		}

		fmt.Fprintln(w)

		var annotated Annotated
		var doc string
		if err := val.Decode(&annotated); err == nil {
			_ = annotated.Meta.GetDecode("doc", &doc)
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
