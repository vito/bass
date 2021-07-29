package bass

import (
	"context"
	"fmt"
)

const TraceSize = 100

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
