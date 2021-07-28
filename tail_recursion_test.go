package bass_test

import (
	"bytes"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
)

func TestTailRecursion(t *testing.T) {
	env := bass.New()

	reader := bytes.NewBufferString(`
		(defn loop []
			(loop))

		(loop)
	`)

	go bass.EvalReader(env, reader)

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

	if !assert.InEpsilon(t, first, last, 10) {
		dump, err := os.CreateTemp("", "heap")
		require.NoError(t, err)

		err = pprof.WriteHeapProfile(dump)
		require.NoError(t, err)

		err = dump.Close()
		require.NoError(t, err)

		t.Logf("wrote heap dump to %s", dump.Name())
	}
}
