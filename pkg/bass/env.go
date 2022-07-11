package bass

import (
	"os"
	"strings"
)

// ImportSystemEnv converts the system env into a scope.
func ImportSystemEnv() *Scope {
	env := NewEmptyScope()

	for _, v := range os.Environ() {
		kv := strings.SplitN(v, "=", 2)
		env.Set(Symbol(kv[0]), String(kv[1]))
	}

	// TODO: this is breaking docker credential helpers; bring it back once
	// that's under control (i.e. we stop overloading docker's auth)
	// os.Clearenv()

	return env
}
