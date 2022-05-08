package internal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/srv"
	"github.com/vito/bass/pkg/zapctx"
)

var Scope *bass.Scope = bass.NewEmptyScope()

// MaxBytes is the maximum size of a request payload.
//
// It is arbitrarily set to 25MB, a value based on GitHub's default payload
// limit.
//
// Bass server servers are not designed to handle unbounded or streaming
// payloads, and sometimes need to buffer the entire request body in order to
// check HMAC signatures, so a reasonable default limit is enforced to help
// prevent DoS attacks.
const MaxBytes = 25 * 1024 * 1024

func init() {
	Scope.Set("string-upper-case",
		bass.Func("string-upper-case", "[str]", strings.ToUpper))

	Scope.Set("string-contains",
		bass.Func("string-contains", "[str substr]", strings.Contains))

	Scope.Set("string-split",
		bass.Func("string-split", "[delim str]", strings.Split))

	Scope.Set("http-listen",
		bass.Func("http-listen", "[addr handler]", func(ctx context.Context, addr string, cb bass.Combiner) error {
			server := &http.Server{
				Addr: addr,
				Handler: http.MaxBytesHandler(srv.Mux(&srv.CallHandler{
					Cb:     cb,
					RunCtx: ctx,
				}), MaxBytes),
			}

			go func() {
				<-ctx.Done()
				// just passing ctx along to immediately interrupt everything
				server.Shutdown(ctx)
			}()

			return server.ListenAndServe()
		}))

	Scope.Set("time-measure",
		bass.Op("time-measure", "[form]", func(ctx context.Context, cont bass.Cont, scope *bass.Scope, form bass.Value) bass.ReadyCont {
			before := bass.Clock.Now()
			return form.Eval(ctx, scope, bass.Continue(func(res bass.Value) bass.Value {
				took := time.Since(before)
				zapctx.FromContext(ctx).Sugar().Debugf("(time %s) => %s took %s", form, res, took)
				return cont.Call(res, nil)
			}))
		}))

	Scope.Set("hmac-verify-sha256",
		bass.Func("hmac-verify-sha256", "[key claim msg]", func(key bass.Secret, claim string, msg []byte) (bool, error) {
			claimSum, err := hex.DecodeString(claim)
			if err != nil {
				return false, err
			}

			mac := hmac.New(sha256.New, key.Reveal())
			_, err = mac.Write(msg)
			if err != nil {
				return false, err
			}

			return hmac.Equal(mac.Sum(nil), claimSum), nil
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
