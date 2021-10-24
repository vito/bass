package internal

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/vito/bass"
	"gopkg.in/yaml.v3"
)

var Scope *bass.Scope = bass.NewEmptyScope()

func init() {
	Scope.Set("string-upper-case", bass.Func("string-upper-case", "[str]", strings.ToUpper))

	Scope.Set("yaml-decode",
		bass.Func("yaml-decode", "[workload-path]", func(ctx context.Context, path bass.WorkloadPath) (bass.Value, error) {
			pool, err := bass.RuntimeFromContext(ctx)
			if err != nil {
				return nil, err
			}

			r, w := io.Pipe()

			go func() {
				w.CloseWithError(pool.Export(ctx, w, path.Workload, path.Path.FilesystemPath()))
			}()

			tr := tar.NewReader(r)

			_, err = tr.Next()
			if err != nil {
				return nil, err
			}

			var v interface{}
			err = yaml.NewDecoder(tr).Decode(&v)
			if err != nil {
				return nil, err
			}

			return bass.ValueOf(v)
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
