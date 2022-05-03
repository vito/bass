package bass

import (
	"context"
	"fmt"
)

const TraceSize = 1000

type Trace struct {
	frames [TraceSize]*Annotate
	depth  int
}

type traceKey struct{}

func (trace *Trace) Record(frame *Annotate) {
	trace.frames[trace.depth%TraceSize] = frame
	trace.depth++
}

func (trace *Trace) Caller(offset int) *Annotate {
	cur := trace.depth - 1 - offset
	if cur < 0 {
		return nil
	}

	return trace.frames[cur%TraceSize]
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

func (trace *Trace) IsEmpty() bool {
	return trace.depth == 0
}

func (trace *Trace) Frames() []*Annotate {
	frames := make([]*Annotate, 0, TraceSize)

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

func (trace *Trace) Reset() {
	trace.depth = 0
}

func WithTrace(ctx context.Context, trace *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, trace)
}

func ForkTrace(ctx context.Context) context.Context {
	if trace, ok := TraceFrom(ctx); ok {
		cp := &Trace{}
		copy(cp.frames[:], trace.frames[:])
		cp.depth = trace.depth
		return context.WithValue(ctx, traceKey{}, cp)
	}

	return ctx
}

func TraceFrom(ctx context.Context) (*Trace, bool) {
	trace := ctx.Value(traceKey{})
	if trace != nil {
		return trace.(*Trace), true
	}

	return nil, false
}

func WithFrame(ctx context.Context, frame *Annotate, cont Cont) Cont {
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

func Caller(ctx context.Context, offset int) Annotate {
	val := ctx.Value(traceKey{})
	if val == nil {
		return Annotate{
			Value: Null{},
		}
	}

	frame := val.(*Trace).Caller(offset)
	if frame != nil {
		return *frame
	}

	return Annotate{
		Value: Null{},
	}
}
