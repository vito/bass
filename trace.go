package bass

import (
	"context"
	"fmt"
)

const history = 100

type Trace struct {
	frames [history]*Annotated
	depth  int
}

type traceKey struct{}

func (trace *Trace) Record(frame *Annotated) {
	trace.frames[trace.depth%history] = frame
	trace.depth++
}

func (trace *Trace) Pop(n int) {
	if trace.depth < n {
		panic(fmt.Sprintf("impossible: popped too far! (%d < %d)", trace.depth, n))
	}

	trace.depth -= n
}

func (trace *Trace) Frames() []*Annotated {
	if trace.depth < history {
		return trace.frames[0:trace.depth]
	}

	frames := make([]*Annotated, 0, history)
	offset := trace.depth % history
	for i := offset; i < history; i++ {
		frames = append(frames, trace.frames[i])
	}
	for i := 0; i < offset; i++ {
		frames = append(frames, trace.frames[i])
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
