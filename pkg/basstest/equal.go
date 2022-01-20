package basstest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vito/bass/pkg/bass"
)

func Equal(t *testing.T, a, b bass.Value) {
	t.Helper()

	if !a.Equal(b) {
		diff := cmp.Diff(a, b)
		t.Logf("%s != %s\n%s", a, b, diff)
		t.FailNow()
	}
}
