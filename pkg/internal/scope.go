package internal

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/zapctx"
)

var Scope *bass.Scope = bass.NewEmptyScope()

func init() {
	Scope.Set("string-upper-case", bass.Func("string-upper-case", "[str]", strings.ToUpper))

	Scope.Set("time-measure",
		bass.Op("time-measure", "[form]", func(ctx context.Context, cont bass.Cont, scope *bass.Scope, form bass.Value) bass.ReadyCont {
			before := bass.Clock.Now()
			return form.Eval(ctx, scope, bass.Continue(func(res bass.Value) bass.Value {
				took := time.Since(before)
				zapctx.FromContext(ctx).Sugar().Debugf("(time %s) => %s took %s", form, res, took)
				return cont.Call(res, nil)
			}))
		}))

	Scope.Set("regexp-case",
		bass.Op("regexp-case", "[str & re-fn-pairs]", func(ctx context.Context, cont bass.Cont, scope *bass.Scope, haystackForm bass.Value, pairs ...bass.Value) bass.ReadyCont {
			if len(pairs)%2 == 1 {
				return cont.Call(nil, fmt.Errorf("unbalanced regexp callback pairs"))
			}

			return haystackForm.Eval(ctx, scope, bass.Continue(func(res bass.Value) bass.Value {
				var str string
				if err := res.Decode(&str); err != nil {
					return cont.Call(nil, err)
				}

				var re *regexp.Regexp
				for i := 0; i < len(pairs); i++ {
					branch := (i / 2) + 1

					v := pairs[i]
					if re == nil {
						var s string
						if err := v.Decode(&s); err != nil {
							return cont.Call(nil, fmt.Errorf("branch %d: %w", branch, err))
						}

						var err error
						re, err = regexp.Compile(s)
						if err != nil {
							return cont.Call(nil, fmt.Errorf("branch %d: %w", branch, err))
						}
					} else {
						matches := re.FindStringSubmatch(str)
						if matches == nil {
							continue
						}

						bindings := bass.Bindings{}
						names := re.SubexpNames()
						for i, v := range matches {
							bindings[bass.Symbol(fmt.Sprintf("$%d", i))] = bass.String(v)

							name := names[i]
							if name != "" {
								bindings[bass.Symbol(fmt.Sprintf("$%s", name))] = bass.String(v)
							}
						}

						return v.Eval(ctx, bindings.Scope(scope), cont)
					}
				}

				// TODO: better error?
				return cont.Call(nil, fmt.Errorf("no branches matched value: %q", str))
			}))
		}))
}
