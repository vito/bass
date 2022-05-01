package cli

import (
	"bytes"
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
	vs  map[digest.Digest]*vertex
	vsL sync.Mutex
}

func NewProgress() *Progress {
	return &Progress{
		vs: map[digest.Digest]*vertex{},
	}
}

func (prog *Progress) WriteStatus(status *graph.SolveStatus) {
	prog.vsL.Lock()
	defer prog.vsL.Unlock()

	for _, v := range status.Vertexes {
		ver, found := prog.vs[v.Digest]
		if !found {
			ver = &vertex{Log: new(bytes.Buffer)}
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
