package bass_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/spy16/slurp/reader"
	"github.com/vito/bass/bass"
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
		t.Run(test.Name, func(t *testing.T) {
			is := is.New(t)

			trace := &bass.Trace{}

			sequential := []*bass.Annotate{}
			for i := 0; i < test.Size; i++ {
				frame := &bass.Annotate{
					Value: bass.Int(i),
					Range: bass.Range{
						Start: reader.Position{File: "test", Ln: i, Col: 1},
						End:   reader.Position{File: "test", Ln: i, Col: 2},
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

func TestTraceWrite(t *testing.T) {
	is := is.New(t)

	trace := &bass.Trace{}

	for i := 0; i < 3; i++ {
		trace.Record(&bass.Annotate{
			Value: bass.Symbol(fmt.Sprintf("call-%d", i+1)),
			Range: bass.Range{
				Start: reader.Position{File: "test", Ln: i + 1, Col: 1},
				End:   reader.Position{File: "test", Ln: i + 1, Col: 2},
			},
		})
	}

	for i := 0; i < 3; i++ {
		trace.Record(&bass.Annotate{
			Value: bass.Symbol(fmt.Sprintf("call-%d", i+1)),
			Range: bass.Range{
				Start: reader.Position{File: "root.bass", Ln: i + 1, Col: 1},
				End:   reader.Position{File: "root.bass", Ln: i + 1, Col: 2},
			},
		})
	}

	trace.Record(&bass.Annotate{
		Value:   bass.Symbol("flake"),
		Comment: "this will fail\nsomeday",
		Range: bass.Range{
			Start: reader.Position{File: "test", Ln: 42, Col: 1},
			End:   reader.Position{File: "test", Ln: 42, Col: 2},
		},
	})

	for i := 0; i < 3; i++ {
		trace.Record(&bass.Annotate{
			Value: bass.Symbol(fmt.Sprintf("call-%d", i+1)),
			Range: bass.Range{
				Start: reader.Position{File: "test", Ln: i + 1, Col: 1},
				End:   reader.Position{File: "test", Ln: i + 1, Col: 2},
			},
		})
	}

	buf := new(bytes.Buffer)
	trace.Write(buf)

	is.Equal(

		buf.String(), strings.Join([]string{
			"\x1b[33merror!\x1b[0m call trace (oldest first):",
			"",
			" 10. test:1\tcall-1",
			"  9. test:2\tcall-2",
			"  8. test:3\tcall-3",
			"\x1b[2m  5. (3 internal calls elided)\x1b[0m",
			"\x1b[2m  4. test:42\t; this will fail\x1b[0m",
			"\x1b[2m  4. test:42\t; someday\x1b[0m",
			"  4. test:42\tflake",
			"  3. test:1\tcall-1",
			"  2. test:2\tcall-2",
			"  1. test:3\tcall-3",
			"",
		}, "\n"))

}
