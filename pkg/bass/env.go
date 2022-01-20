package bass

import (
	"os"
	"strings"
)

// ImportSystemEnv converts the system env into a scope, clearing the system
// env in the process. This is a destructive operation.
func ImportSystemEnv() *Scope {
	env := NewEmptyScope()

	for _, v := range os.Environ() {
		kv := strings.SplitN(v, "=", 2)
		env.Set(Symbol(kv[0]), String(kv[1]))
	}

	os.Clearenv()

	return env
}
