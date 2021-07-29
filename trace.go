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

func WithTrace(ctx context.Context) (context.Context, *Trace) {
	val := ctx.Value(traceKey{})
	if val != nil {
		return ctx, val.(*Trace)
	}

	val = &Trace{}

	return context.WithValue(ctx, traceKey{}, val), val.(*Trace)
}

func WithFrame(frame *Annotated, ctx context.Context) (context.Context, *Trace) {
	val := ctx.Value(traceKey{})
	if val == nil {
		val = &Trace{}
		ctx = context.WithValue(ctx, traceKey{}, val)
	}

	trace := val.(*Trace)

	// update in-place to avoid needing to always allocate a new context.Context
	//
	// each goroutine _must_ have a separate Trace
	//
	// TODO: consider indicating relationship/starting from snapshot of trace?
	trace.Record(frame)

	return ctx, trace
}

func TraceFrom(ctx context.Context) *Trace {
	val := ctx.Value(traceKey{})
	if val == nil {
		val = &Trace{}
	}

	return val.(*Trace)
}
