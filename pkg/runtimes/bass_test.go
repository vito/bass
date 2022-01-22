package runtimes_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/runtimes"
	"github.com/vito/is"
)

func TestBass(t *testing.T) {
	is := is.New(t)

	pool, err := runtimes.NewPool(&bass.Config{})
	is.NoErr(err)

	for _, test := range []struct {
		File     string
		Result   bass.Value
		Bindings bass.Bindings
	}{
		{
			File:   "bass/run.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "bass/load.bass",
			Result: bass.NewList(bass.Int(1), bass.Int(2), bass.Int(3)),
		},
		{
			File:   "bass/use.bass",
			Result: bass.String("61,2,3"),
		},
		{
			File:   "bass/env.bass",
			Result: bass.NewList(bass.String("123"), bass.String("123")),
		},
	} {
		test := test
		t.Run(filepath.Base(test.File), func(t *testing.T) {
			is := is.New(t)

			t.Parallel()

			res, err := runtimes.RunTest(context.Background(), t, pool, test.File, nil)
			is.NoErr(err)
			is.True(res != nil)
			Equal(t, res, test.Result)
		})
	}
}
