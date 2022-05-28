package internal_test

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/internal"
	"github.com/vito/bass/pkg/ioctx"
	"github.com/vito/bass/pkg/zapctx"
	"github.com/vito/is"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

func TestInternalBindings(t *testing.T) {
	for _, example := range []BasicExample{
		{
			Name:   "time-measure",
			Bass:   `(time-measure (dump 42))`,
			Result: bass.Int(42),
			Log:    []string{`DEBUG\t\(time \(dump 42\)\) => 42 took \d.+s`},
		},
	} {
		t.Run(example.Name, example.Run)
	}
}

type BasicExample struct {
	Name string

	Bind bass.Bindings
	Bass string

	Result           bass.Value
	Meta             *bass.Scope
	ResultConsistsOf bass.List
	Binds            bass.Bindings

	Stderr string
	Log    []string

	Err         error
	ErrEqual    error
	ErrContains string
}

func (example BasicExample) Run(t *testing.T) {
	t.Run(example.Name, func(t *testing.T) {
		is := is.New(t)

		scope := bass.NewEmptyScope(bass.NewStandardScope(), internal.Scope)

		if example.Bind != nil {
			for k, v := range example.Bind {
				scope.Set(k, v)
			}
		}

		ctx := context.Background()

		stderrBuf := new(bytes.Buffer)
		ctx = ioctx.StderrToContext(ctx, stderrBuf)

		zapcfg := zap.NewDevelopmentEncoderConfig()
		zapcfg.EncodeTime = nil

		logBuf := new(zaptest.Buffer)
		logger := zap.New(zapcore.NewCore(
			zapcore.NewConsoleEncoder(zapcfg),
			zapcore.AddSync(logBuf),
			zapcore.DebugLevel,
		))

		ctx = zapctx.ToContext(ctx, logger)

		reader := bass.NewInMemoryFile("test", example.Bass)
		res, err := bass.EvalFSFile(ctx, scope, reader)

		if example.Err != nil {
			is.True(errors.Is(err, example.Err))
		} else if example.ErrEqual != nil {
			is.True(err.Error() == example.ErrEqual.Error())
		} else if example.ErrContains != "" {
			is.True(err != nil)
			is.True(strings.Contains(err.Error(), example.ErrContains))
		} else if example.ResultConsistsOf != nil {
			is.NoErr(err)

			expected, err := bass.ToSlice(example.ResultConsistsOf)
			is.NoErr(err)

			var actualList bass.List
			err = res.Decode(&actualList)
			is.NoErr(err)

			actual, err := bass.ToSlice(actualList)
			is.NoErr(err)

			is.True(cmp.Equal(actual, expected, cmpopts.SortSlices(func(a, b bass.Value) bool {
				return a.String() < b.String()
			})))
		} else {
			is.NoErr(err)

			if example.Result != nil {
				if example.Meta != nil {
					var ann bass.Annotated
					err := res.Decode(&ann)
					is.NoErr(err)

					if !example.Meta.IsSubsetOf(ann.Meta) {
						t.Errorf("meta: %s ⊄ %s\n%s", example.Meta, ann.Meta, cmp.Diff(example.Meta, ann.Meta))
					}
				}

				if !res.Equal(example.Result) {
					t.Errorf("%s != %s\n%s", res, example.Result, cmp.Diff(res, example.Result))
				}
			} else if example.Binds != nil {
				is.Equal(example.Binds, scope.Bindings)
			}
		}

		if example.Stderr != "" {
			is.Equal(stderrBuf.String(), example.Stderr)
		}

		if example.Log != nil {
			lines := logBuf.Lines()
			is.True(len(lines) == len(example.Log))

			for i, l := range example.Log {
				logRe, err := regexp.Compile(l)
				is.NoErr(err)
				is.True(logRe.MatchString(lines[i]))
			}
		}
	})
}
