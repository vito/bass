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

	dir := state.Dir
	if dir == nil {
		dir = bass.DirPath{Path: "."}
	}

	scope.Set("*dir*", dir, `current working directory`,
		`This value is always set to the directory containing the script being run.`,
		`It can and should be used to load sibling/child paths, e.g. *dir*/foo to load the 'foo.bass' file in the same directory as the current file.`)

	var env *bass.Scope
	if state.Env == nil {
		env = bass.NewEmptyScope()
	} else {
		env = state.Env.Copy()
	}
	scope.Set("*env*", env, `environment variables`,
		`System environment variables are only available to the entrypoint script. To propagate them further they must be explicitly passed to thunks using (with-env).`,
		`System environment variables are unset from the physical OS process as part of initialization to ensure they cannot be leaked.`)

	stdin := state.Stdin
	if stdin == nil {
		stdin = bass.NewSource(bass.NewInMemorySource())
	}
	scope.Set("*stdin*", stdin, `standard input stream`,
		`Values read from *stdin* will be parsed from the process's stdin as a JSON stream.`)

	stdout := state.Stdout
	if stdout == nil {
		stdout = bass.NewSink(bass.NewInMemorySink())
	}
	scope.Set("*stdout*", stdout, `standard output sink`,
		`Values emitted by a script to *stdout* will be encoded as a JSON stream to the process's stdout.`)

	scope.Set("main", bass.Func("main", "[]", func() {}),
		`script entrypoint`,
		`The (main) function is called with any provided command-line args when running a Bass script.`,
		`Scripts should define it to capture system arguments and run the script's desired effects.`,
		`Putting effects in (main) instead of running them at the toplevel makes the Bass language server happier.`)

	return bass.NewEmptyScope(scope)
}
