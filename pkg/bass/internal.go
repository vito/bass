package bass

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/vito/bass/pkg/zapctx"
)

var Internal *Scope = NewEmptyScope()

func init() {
	Internal.Set("string-upper-case",
		Func("string-upper-case", "[str]", strings.ToUpper))

	Internal.Set("string-contains",
		Func("string-contains", "[str substr]", strings.Contains))

	Internal.Set("string-split",
		Func("string-split", "[delim str]", strings.Split))

	Internal.Set("time-measure",
		Op("time-measure", "[form]", func(ctx context.Context, cont Cont, scope *Scope, form Value) ReadyCont {
			before := Clock.Now()
			return form.Eval(ctx, scope, Continue(func(res Value) Value {
				took := time.Since(before)
				zapctx.FromContext(ctx).Sugar().Debugf("(time %s) => %s took %s", form.Repr(), res.Repr(), took)
				return cont.Call(res, nil)
			}))
		}))

	Internal.Set("regexp-case",
		Op("regexp-case", "[str & re-fn-pairs]", func(ctx context.Context, cont Cont, scope *Scope, haystackForm Value, pairs ...Value) ReadyCont {
			if len(pairs)%2 == 1 {
				return cont.Call(nil, fmt.Errorf("unbalanced regexp callback pairs"))
			}

			return haystackForm.Eval(ctx, scope, Continue(func(res Value) Value {
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

						bindings := Bindings{}
						names := re.SubexpNames()
						for i, v := range matches {
							bindings[Symbol(fmt.Sprintf("$%d", i))] = String(v)

							name := names[i]
							if name != "" {
								bindings[Symbol(fmt.Sprintf("$%s", name))] = String(v)
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
