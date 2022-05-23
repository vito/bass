package cli

import (
	"bytes"
	"io"
	"sync"

	"github.com/morikuni/aec"
	"github.com/opencontainers/go-digest"
	"github.com/vito/progrock/graph"
	"github.com/vito/progrock/ui"
)

var ProgressUI = ui.Default

func init() {
	ProgressUI.ConsoleRunning = "Playing %s (%d/%d)"
	ProgressUI.ConsoleDone = "Playing %s (%d/%d) " + aec.GreenF.Apply("done")
}

type Progress struct {
	vs  map[digest.Digest]*Vertex
	vsL sync.Mutex
}

type Vertex struct {
	*graph.Vertex

	Log *bytes.Buffer
}

func NewProgress() *Progress {
	return &Progress{
		vs: map[digest.Digest]*Vertex{},
	}
}

func (prog *Progress) EachVertex(f func(*Vertex) error) error {
	for _, v := range prog.vs {
		err := f(v)
		if err != nil {
			return err
		}
	}

	return nil
}

func (prog *Progress) WriteStatus(status *graph.SolveStatus) {
	prog.vsL.Lock()
	defer prog.vsL.Unlock()

	for _, v := range status.Vertexes {
		ver, found := prog.vs[v.Digest]
		if !found {
			ver = &Vertex{Log: new(bytes.Buffer)}
			prog.vs[v.Digest] = ver
		}

		ver.Vertex = v
	}

	for _, l := range status.Logs {
		ver, found := prog.vs[l.Vertex]
		if !found {
			continue
		}

		_, _ = ver.Log.Write(l.Data)
	}
}

func (prog *Progress) Close() {}

func (prog *Progress) WrapError(msg string, err error) *ProgressError {
	return &ProgressError{
		msg:  msg,
		err:  err,
		prog: prog,
	}
}

func (prog *Progress) Summarize(w io.Writer) {
	vtxPrinter{
		vs:      prog.vs,
		printed: map[digest.Digest]struct{}{},
	}.printAll(w)
}
