package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/is"
)

func TestTrace(t *testing.T) {
	for _, test := range []struct {
		Name string
		Size int
		Pop  int
	}{
		{
			Name: "one",
			Size: 1,
			Pop:  1,
		},
		{
			Name: "half",
			Size: bass.TraceSize / 2,
			Pop:  10,
		},
		{
			Name: "full",
			Size: bass.TraceSize,
			Pop:  10,
		},
		{
			Name: "minus one",
			Size: bass.TraceSize - 1,
			Pop:  10,
		},
		{
			Name: "plus one",
			Size: bass.TraceSize + 1,
			Pop:  10,
		},
		{
			Name: "1.5x",
			Size: bass.TraceSize + (bass.TraceSize / 2),
			Pop:  10,
		},
		{
			Name: "2x",
			Size: bass.TraceSize * 2,
			Pop:  10,
		},
		{
			Name: "100x",
			Size: bass.TraceSize * 100,
			Pop:  10,
		},
		{
			Name: "100x pop half",
			Size: bass.TraceSize * 100,
			Pop:  bass.TraceSize / 2,
		},
		{
			Name: "100x pop full",
			Size: bass.TraceSize * 100,
			Pop:  bass.TraceSize,
		},
		{
			Name: "100x pop 2x",
			Size: bass.TraceSize * 100,
			Pop:  bass.TraceSize * 2,
		},
	} {
		file := bass.NewInMemoryFile("test", "")

		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)

			trace := &bass.Trace{}

			sequential := []*bass.Annotate{}
			for i := 0; i < test.Size; i++ {
				frame := &bass.Annotate{
					Value: bass.Int(i),
					Range: bass.Range{
						File:  file,
						Start: bass.Position{Ln: i, Col: 1},
						End:   bass.Position{Ln: i, Col: 2},
					},
				}

				trace.Record(frame)

				sequential = append(sequential, frame)
			}

			var start int
			if test.Size > bass.TraceSize {
				start = (test.Size - bass.TraceSize) % test.Size
			}

			is.Equal(trace.Frames(), sequential[start:])

			trace.Pop(test.Pop)

			remaining := bass.TraceSize - test.Pop
			if remaining < 0 {
				remaining = 0
			}

			if test.Size > bass.TraceSize {
				is.True(len(trace.Frames()) == remaining)
			} else {
				is.True(len(trace.Frames()) == test.Size-test.Pop)
			}

			if remaining > 0 {
				is.Equal(trace.Frames(), sequential[start:(test.Size-test.Pop)])
			}
		})
	}
}
