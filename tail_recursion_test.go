package bass_test

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/vito/bass"
	"github.com/vito/is"
)

func TestTailRecursion(t *testing.T) {
	is := is.New(t)

	if testing.Short() {
		t.Skip("skipping slow test")
		return
	}

	scope := bass.NewStandardScope()

	reader := bytes.NewBufferString(`
		(defn loop [val]
			(loop val))

		(loop "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bass.EvalReader(ctx, scope, reader)

	time.Sleep(10 * time.Millisecond)

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	first := stats.HeapObjects
	last := first

	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)

		runtime.ReadMemStats(&stats)

		cur := stats.HeapObjects
		t.Logf("heap objects: %d", cur)

		last = cur
	}

	ratio := float64(last) / float64(first)
	t.Logf("heap growth ratio: %d -> %d = (%.2fx)", first, last, ratio)

	if ratio > 2 {
		dump, err := os.Create("TestTailRecursion.out")
		is.NoErr(err)

		err = pprof.WriteHeapProfile(dump)
		is.NoErr(err)

		err = dump.Close()
		is.NoErr(err)

		t.Logf("heap before: %d, after: %d; wrote heap dump to %s", first, last, dump.Name())
		is.Fail()
	}
}
