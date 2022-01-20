package runtimes

import (
	"github.com/vito/bass/bass"
)

type RunState struct {
	Dir    bass.Path
	Args   bass.List
	Stdin  *bass.Source
	Stdout *bass.Sink
	Env    *bass.Scope
}

func NewScope(parent *bass.Scope, state RunState) *bass.Scope {
	scope := bass.NewEmptyScope(parent)

	scope.Set("*dir*",
		state.Dir,
		`working directory`,
		`This value is always set to the directory containing the file being run.`,
		`It can and should be used to load sibling/child paths, e.g. *dir*/foo to load the 'foo.bass' file in the same directory as the current file.`)

	scope.Set("*args*", state.Args, `command line arguments`)

	scope.Set("*stdin*", state.Stdin, `standard input stream`)

	scope.Set("*stdout*", state.Stdout, `standard output sink`)

	var env *bass.Scope
	if state.Env == nil {
		env = bass.NewEmptyScope()
	} else {
		env = bass.NewScope(bass.Bindings{}, state.Env)
	}
	scope.Set("*env*", env, `environment variables`)

	return bass.NewEmptyScope(scope)
}
