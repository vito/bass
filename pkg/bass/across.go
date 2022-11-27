package bass

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/go-multierror"
)

type CrossSource struct {
	sources []*Source
	cases   []reflect.SelectCase
	chans   []<-chan Value
	next    []Value
	close   func()
}

func Across(ctx context.Context, sources ...*Source) *Source {
	ctx, cancel := context.WithCancel(ctx)

	agg := &CrossSource{
		sources: sources,
		cases:   make([]reflect.SelectCase, len(sources)),
		chans:   make([]<-chan Value, len(sources)),
		next:    make([]Value, len(sources)),
		close:   cancel,
	}

	for i, src := range sources {
		ch := make(chan Value)
		agg.chans[i] = ch
		agg.cases[i] = reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch),
		}

		go agg.update(ctx, src.PipeSource, ch)
	}

	return &Source{agg}
}

func (cross *CrossSource) String() string {
	return fmt.Sprintf("<cross: %v>", cross.sources)
}

func (cross *CrossSource) Close() error {
	cross.close()

	var errs error
	for _, src := range cross.sources {
		if err := src.PipeSource.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

func (cross *CrossSource) update(ctx context.Context, stream PipeSource, ch chan<- Value) {
	for {
		obj, err := stream.Next(ctx)
		if err != nil {
			close(ch)
			return
		}

		select {
		case ch <- obj:
		case <-ctx.Done():
			return
		}
	}
}

func (cross *CrossSource) Next(ctx context.Context) (Value, error) {
	if len(cross.chans) == 0 {
		return nil, ErrEndOfSource
	}

	updated := false
	for i, ch := range cross.chans {
		if cross.next[i] != nil {
			continue
		}

		select {
		case val, ok := <-ch:
			if !ok {
				return nil, ErrEndOfSource
			}

			cross.next[i] = val
			updated = true

		case <-ctx.Done():
			return nil, ErrInterrupted
		}
	}

	if updated {
		return NewList(cross.next...), nil
	}

	cases := cross.cases

	doneIdx := len(cases)
	cases = append(cases, reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(ctx.Done()),
	})

	defaultIdx := len(cases)
	cases = append(cases, reflect.SelectCase{
		Dir: reflect.SelectDefault,
	})

	hasNew := false
	hasDefault := true
	exhausted := 0

	for {
		idx, val, recvOK := reflect.Select(cases)
		if idx == doneIdx { // ctx.Done()
			return nil, ErrInterrupted
		}

		if hasDefault && idx == defaultIdx {
			if hasNew {
				return NewList(cross.next...), nil
			}

			// nothing new, remove the default so we block on an update instead
			cases = append(cases[:idx], cases[idx+1:]...)
			hasDefault = false
			continue
		}

		if !recvOK {
			exhausted++

			cases[idx] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(nil),
			}

			if exhausted == len(cross.cases) {
				// all sources have run dry
				return nil, ErrEndOfSource
			}

			continue
		}

		cross.next[idx] = val.Interface().(Value)

		if hasDefault {
			hasNew = true

			// nil out the channel so we don't skip values while collecting from the
			// other sources
			cases[idx] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(nil),
			}
		} else {
			return NewList(cross.next...), nil
		}
	}
}
