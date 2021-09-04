package bass

import (
	"context"
	"fmt"
	"strings"

	"github.com/vito/bass/ioctx"
)

var separator = fmt.Sprintf("\x1b[90m%s\x1b[0m", strings.Repeat("-", 50))

func PrintDocs(ctx context.Context, scope *Scope, kws ...Keyword) {
	w := ioctx.StderrFromContext(ctx)

	if len(kws) == 0 {
		for _, comment := range scope.Commentary {
			fmt.Fprintln(w, separator)

			var sym Symbol
			if err := comment.Value.Decode(&sym); err == nil {
				PrintBindingDocs(ctx, scope, sym.Keyword())
				continue
			}

			fmt.Fprintln(w, comment.Comment)
			if comment.Value != (Ignore{}) {
				fmt.Fprintln(w, comment.Value)
			}

			fmt.Fprintln(w)
		}

		return
	}

	for _, kw := range kws {
		fmt.Fprintln(w, separator)
		PrintBindingDocs(ctx, scope, kw)
	}
}

// Predicates returns a list of all builtin predicates which return true for
// the given value.
func Predicates(val Value) []Symbol {
	var preds []Symbol
	for _, pred := range primPreds {
		if pred.check(val) {
			preds = append(preds, pred.name.Symbol())
		}
	}

	return preds
}

func PrintBindingDocs(ctx context.Context, scope *Scope, kw Keyword) {
	w := ioctx.StderrFromContext(ctx)

	fmt.Fprintf(w, "\x1b[32m%s\x1b[0m", kw.Symbol())

	annotated, found := scope.GetWithDoc(kw)
	if !found {
		fmt.Fprintf(w, " \x1b[31msymbol not bound\x1b[0m\n")
	} else {
		val := annotated.Value

		for _, pred := range Predicates(val) {
			fmt.Fprintf(w, " \x1b[33m%s\x1b[0m", pred)
		}

		fmt.Fprintln(w)

		var app Applicative
		if err := val.Decode(&app); err == nil {
			val = app.Unwrap()
		}

		var operative *Operative
		if err := val.Decode(&operative); err == nil {
			fmt.Fprintln(w, "args:", operative.Formals)
		}

		var builtin *Builtin
		if err := val.Decode(&builtin); err == nil {
			fmt.Fprintln(w, "args:", builtin.Formals)
		}
	}

	doc := scope.Docs[kw].Comment

	if doc != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, doc)
	}

	fmt.Fprintln(w)
}
