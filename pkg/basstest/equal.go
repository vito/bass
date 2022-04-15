package basstest

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/vito/bass/pkg/bass"
)

func Equal(t *testing.T, a, b bass.Value) {
	t.Helper()

	if !a.Equal(b) {
		diff := tryDiff(a, b)
		t.Logf("%s != %s\n%s", a.Repr(), b.Repr(), diff)
		t.FailNow()
	}
}

func tryDiff(a, b any) (res string) {
	defer func() {
		// cmp panics if equal is asymmetrical; recover for better failure ux
		err := recover()
		if err != nil {
			res = fmt.Sprintf("diff error: %s", err)
		}
	}()

	return cmp.Diff(a, b)
}
