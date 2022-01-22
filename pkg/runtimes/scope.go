package runtimes

import (
	"github.com/vito/bass/pkg/bass"
)

type RunState struct {
	Dir    bass.Path
	Env    *bass.Scope
	Stdin  *bass.Source
	Stdout *bass.Sink
}

func NewScope(parent *bass.Scope, state RunState) *bass.Scope {
	scope := bass.NewEmptyScope(parent)

	scope.Set("*dir*",
		state.Dir,
		`working directory`,
		`This value is always set to the directory containing the file being run.`,
		`It can and should be used to load sibling/child paths, e.g. *dir*/foo to load the 'foo.bass' file in the same directory as the current file.`)

	var env *bass.Scope
	if state.Env == nil {
		env = bass.NewEmptyScope()
	} else {
		env = bass.NewScope(bass.Bindings{}, state.Env)
	}
	scope.Set("*env*", env, `environment variables`)

	scope.Set("*stdin*", state.Stdin, `standard input stream`)

	scope.Set("*stdout*", state.Stdout, `standard output sink`)

	return bass.NewEmptyScope(scope)
}
