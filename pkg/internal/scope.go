package internal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/zapctx"
)

var Scope *bass.Scope = bass.NewEmptyScope()

type timeSeries struct {
	interval time.Duration
	initial  int32
}

func newTimeSeries(interval time.Duration) bass.PipeSource {
	return &timeSeries{interval: interval}
}

func (series *timeSeries) String() string {
	return fmt.Sprintf("(series: %s)", series.interval)
}

func (series *timeSeries) Next(ctx context.Context) (bass.Value, error) {
	now := time.Now()

	cur := now.Truncate(series.interval)
	if atomic.CompareAndSwapInt32(&series.initial, 0, 1) {
		// return timestamp immediately once
		return bass.String(cur.UTC().Format(time.RFC3339)), nil
	}

	next := cur.Add(series.interval)
	select {
	case <-time.After(next.Sub(now)):
		return bass.String(next.UTC().Format(time.RFC3339)), nil
	case <-ctx.Done():
		return nil, bass.ErrInterrupted
	}
}

func init() {
	Scope.Set("string-upper-case",
		bass.Func("string-upper-case", "[str]", strings.ToUpper))

	Scope.Set("string-contains",
		bass.Func("string-contains", "[str substr]", strings.Contains))

	Scope.Set("string-split",
		bass.Func("string-split", "[delim str]", strings.Split))

	Scope.Set("time-measure",
		bass.Op("time-measure", "[form]", func(ctx context.Context, cont bass.Cont, scope *bass.Scope, form bass.Value) bass.ReadyCont {
			before := bass.Clock.Now()
			return form.Eval(ctx, scope, bass.Continue(func(res bass.Value) bass.Value {
				took := time.Since(before)
				zapctx.FromContext(ctx).Sugar().Debugf("(time %s) => %s took %s", form, res, took)
				return cont.Call(res, nil)
			}))
		}))

	Scope.Set("time-series",
		bass.Func("time-series", "[interval]", func(interval int) *bass.Source {
			return bass.NewSource(newTimeSeries(time.Duration(interval) * time.Second))
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

			log.Println("HMAC", hex.EncodeToString(mac.Sum(nil)), claim, hex.EncodeToString(claimSum))

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
