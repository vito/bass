package bass

import (
	"context"
	"fmt"
	"io"
	"strings"
)

const TraceSize = 1000

type Trace struct {
	frames [TraceSize]*Annotated
	depth  int
}

type traceKey struct{}

func (trace *Trace) Record(frame *Annotated) {
	trace.frames[trace.depth%TraceSize] = frame
	trace.depth++
}

func (trace *Trace) Pop(n int) {
	if trace.depth < n {
		panic(fmt.Sprintf("impossible: popped too far! (%d < %d)", trace.depth, n))
	}

	for i := 0; i < n; i++ {
		trace.depth--
		trace.frames[trace.depth%TraceSize] = nil
	}
}

func (trace *Trace) Frames() []*Annotated {
	frames := make([]*Annotated, 0, TraceSize)

	offset := trace.depth % TraceSize
	for i := offset; i < TraceSize; i++ {
		frame := trace.frames[i]
		if frame == nil {
			continue
		}

		frames = append(frames, frame)
	}

	for i := 0; i < offset; i++ {
		frame := trace.frames[i]
		if frame == nil {
			continue
		}

		frames = append(frames, frame)
	}

	return frames
}

const ExprLen = 40

func (trace *Trace) Write(out io.Writer) {
	frames := trace.Frames()

	fmt.Fprintf(out, "\x1b[33merror!\x1b[0m call trace (oldest first):\n\n")

	elided := 0
	for i, frame := range frames {
		if frame.Range.Start.File == internalName {
			elided++
			continue
		}

		num := len(frames) - i
		if elided > 0 {
			if elided == 1 {
				fmt.Fprintf(out, "\x1b[2m%3d. (1 internal call elided)\x1b[0m\n", num+1)
			} else {
				fmt.Fprintf(out, "\x1b[2m%3d. (%d internal calls elided)\x1b[0m\n", num+1, elided)
			}

			elided = 0
		}

		expr := frame.Value.String()
		if len(expr) > ExprLen {
			expr = expr[:ExprLen-3]
			expr += "..."
		}

		prefix := fmt.Sprintf("%3d. %s:%d", num, frame.Range.Start.File, frame.Range.Start.Ln)

		if frame.Comment != "" {
			for _, line := range strings.Split(frame.Comment, "\n") {
				fmt.Fprintf(out, "\x1b[2m%s\t; %s\x1b[0m\n", prefix, line)
			}
		}

		fmt.Fprintf(out, "%s\t%s\n", prefix, expr)
	}
}

func (trace *Trace) Reset() {
	trace.depth = 0
}

func WriteError(ctx context.Context, out io.Writer, err error) {
	val := ctx.Value(traceKey{})
	if val != nil {
		trace := val.(*Trace)
		trace.Write(Stderr)
		trace.Reset()
		fmt.Fprintln(Stderr)
	}

	fmt.Fprintf(Stderr, "\x1b[31m%s\x1b[0m\n", err)
}

func WithTrace(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

func WithFrame(frame *Annotated, ctx context.Context, cont Cont) Cont {
	val := ctx.Value(traceKey{})
	if val == nil {
		return cont
	}

	trace := val.(*Trace)

	// update in-place to avoid needing to always allocate a new context.Context
	//
	// each goroutine _must_ have a separate Trace
	//
	// TODO: consider indicating relationship/starting from snapshot of trace?
	trace.Record(frame)

	return cont.Traced(trace)
}
