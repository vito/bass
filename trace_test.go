package bass_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
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
		t.Run(test.Name, func(t *testing.T) {
			trace := &bass.Trace{}

			sequential := []*bass.Annotated{}
			for i := 0; i < test.Size; i++ {
				frame := &bass.Annotated{
					Value: bass.Int(i),
				}

				trace.Record(frame)

				sequential = append(sequential, frame)
			}

			var start int
			if test.Size > bass.TraceSize {
				start = (test.Size - bass.TraceSize) % test.Size
			}

			require.Equal(t, sequential[start:], trace.Frames())

			trace.Pop(test.Pop)

			remaining := bass.TraceSize - test.Pop
			if remaining < 0 {
				remaining = 0
			}

			if test.Size > bass.TraceSize {
				require.Len(t, trace.Frames(), remaining)
			} else {
				require.Len(t, trace.Frames(), test.Size-test.Pop)
			}

			if remaining > 0 {
				require.Equal(t, sequential[start:(test.Size-test.Pop)], trace.Frames())
			}
		})
	}
}
