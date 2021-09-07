package prog

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/moby/buildkit/client"
	"github.com/opencontainers/go-digest"
)

// Clock is used to determine the current time.
var Clock = clockwork.NewRealClock()

type Recorder struct {
	Source chan *client.SolveStatus

	dest     chan<- *client.SolveStatus
	vertexes map[digest.Digest]*VertexRecorder
}

func NewRecorder() *Recorder {
	ch := make(chan *client.SolveStatus)

	return &Recorder{
		Source: ch,

		dest:     ch,
		vertexes: map[digest.Digest]*VertexRecorder{},
	}
}

func (recorder *Recorder) Record(status *client.SolveStatus) {
	if recorder.dest == nil {
		// noop
		return
	}

	for i, v := range status.Vertexes {
		cp := *v
		status.Vertexes[i] = &cp
	}

	for i, v := range status.Statuses {
		cp := *v
		status.Statuses[i] = &cp
	}

	recorder.dest <- status
}

func (recorder *Recorder) Close() {
	close(recorder.dest)
}

type recorderKey struct{}

func RecorderToContext(ctx context.Context, recorder *Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, recorder)
}

func RecorderFromContext(ctx context.Context) *Recorder {
	rec := ctx.Value(recorderKey{})
	if rec == nil {
		noop := NewRecorder()
		noop.dest = nil
		return noop // throwaway
	}

	return rec.(*Recorder)
}

type VertexRecorder struct {
	Vertex   *client.Vertex
	Recorder *Recorder

	statuses map[string]*TaskRecorder
}

func (recorder *Recorder) Vertex(dig digest.Digest, name string) *VertexRecorder {
	rec, found := recorder.vertexes[dig]
	if !found {
		now := Clock.Now()

		rec = &VertexRecorder{
			Recorder: recorder,

			Vertex: &client.Vertex{
				Digest: dig,
				Name:   name,

				Started: &now,
			},

			statuses: map[string]*TaskRecorder{},
		}

		recorder.vertexes[dig] = rec
	}

	rec.sync()

	return rec
}

func (recorder *VertexRecorder) Stdout() io.Writer {
	return &recordWriter{
		Stream:         1,
		VertexRecorder: recorder,
	}
}

func (recorder *VertexRecorder) Stderr() io.Writer {
	return &recordWriter{
		Stream:         2,
		VertexRecorder: recorder,
	}
}

func (recorder *VertexRecorder) Complete() {
	now := Clock.Now()

	if recorder.Vertex.Completed == nil {
		// could already be set if referenced by a downstream workload
		recorder.Vertex.Completed = &now
	}

	recorder.sync()
}

func (recorder *VertexRecorder) Error(err error) {
	recorder.Vertex.Error = err.Error()
	recorder.sync()
}

func (recorder *VertexRecorder) Cached() {
	if recorder.Vertex.Completed != nil {
		// referenced again by another workload
		return
	}

	recorder.Vertex.Cached = true
	recorder.sync()
}

func (recorder *VertexRecorder) sync() {
	recorder.Recorder.Record(&client.SolveStatus{
		Vertexes: []*client.Vertex{
			recorder.Vertex,
		},
	})
}

func (recorder *VertexRecorder) Task(msg string, args ...interface{}) *TaskRecorder {
	id := fmt.Sprintf(msg, args...)

	task, found := recorder.statuses[id]
	if !found {
		now := Clock.Now()
		task = &TaskRecorder{
			Status: &client.VertexStatus{
				ID:        id,
				Vertex:    recorder.Vertex.Digest,
				Name:      "?name?: " + id, // unused/deprecated?
				Timestamp: now,
			},
			VertexRecorder: recorder,
		}

		recorder.statuses[id] = task
	}

	task.sync()

	return task
}

type TaskRecorder struct {
	*VertexRecorder

	Status *client.VertexStatus
}

func (recorder *TaskRecorder) Wrap(f func() error) error {
	recorder.Start()

	err := f()
	recorder.Done(err)

	return err
}

func (recorder *TaskRecorder) Done(err error) {
	if err != nil {
		recorder.Error(err)
	}

	recorder.Complete()
}

func (recorder *TaskRecorder) Start() {
	now := Clock.Now()
	recorder.Status.Started = &now
	recorder.sync()
}

func (recorder *TaskRecorder) Complete() {
	now := Clock.Now()

	if recorder.Status.Started == nil {
		recorder.Status.Started = &now
	}

	recorder.Status.Completed = &now
	recorder.sync()
}

func (recorder *TaskRecorder) Progress(cur, total int64) {
	recorder.Status.Current = cur
	recorder.Status.Total = total
	recorder.sync()
}

func (recorder *TaskRecorder) sync() {
	recorder.Recorder.Record(&client.SolveStatus{
		Statuses: []*client.VertexStatus{
			recorder.Status,
		},
	})
}

type recordWriter struct {
	*VertexRecorder

	Stream int
}

func (w *recordWriter) Write(b []byte) (int, error) {
	d := make([]byte, len(b))
	copy(d, b)

	w.Recorder.Record(&client.SolveStatus{
		Logs: []*client.VertexLog{
			{
				Vertex:    w.Vertex.Digest,
				Stream:    w.Stream,
				Data:      d,
				Timestamp: time.Now(), // XXX: omit?
			},
		},
	})

	return len(b), nil
}
