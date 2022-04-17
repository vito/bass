package bass_test

import (
	"context"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestAcross(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, example := range []struct {
		Name    string
		Sources [][]bass.Value
	}{
		{
			Name: "empty",
		},
		{
			Name: "two sources",
			Sources: [][]bass.Value{
				{bass.Int(0), bass.Int(2), bass.Int(4)},
				{bass.Int(1), bass.Int(3), bass.Int(5)},
			},
		},
		{
			Name: "two sources, imbalanced",
			Sources: [][]bass.Value{
				{
					bass.Int(0),
					bass.Int(2),
					bass.Int(4),
					bass.Int(6),
					bass.Int(8),
					bass.Int(10),
					bass.Int(12),
				},
				{bass.Int(1), bass.Int(3), bass.Int(5)},
			},
		},
		{
			Name: "three sources",
			Sources: [][]bass.Value{
				{bass.Int(0), bass.Int(2), bass.Int(4)},
				{bass.Int(1), bass.Int(3), bass.Int(5)},
				{bass.Symbol("one"), bass.Symbol("three"), bass.Symbol("five")},
			},
		},
	} {
		t.Run(example.Name, func(t *testing.T) {
			is := is.New(t)

			srcs := make([]*bass.Source, len(example.Sources))
			for i, vs := range example.Sources {
				srcs[i] = bass.NewSource(bass.NewInMemorySource(vs...))
			}

			src := bass.Across(ctx, srcs...)

			have := make([][]bass.Value, len(example.Sources))
			for {
				val, err := src.PipeSource.Next(ctx)
				t.Logf("next: %v %v", val, err)
				if err == bass.ErrEndOfSource {
					break
				}

				is.NoErr(err)

				vals, err := bass.ToSlice(val.(bass.List))
				is.NoErr(err)

				for i, v := range vals {
					seen := len(have[i])
					if seen == 0 || have[i][seen-1] != v {
						have[i] = append(have[i], v)
					}
				}
			}

			for i, vals := range example.Sources {
				t.Logf("saw from %d: %v = %v", i, vals, have[i])
				is.Equal(vals, have[i])
			}
		})
	}
}
