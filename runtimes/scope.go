package runtimes

import (
	"github.com/vito/bass"
)

type RunState struct {
	Dir    bass.Path
	Args   bass.List
	Stdin  *bass.Source
	Stdout *bass.Sink
}

func NewScope(parent *bass.Scope, state RunState) *bass.Scope {
	scope := bass.NewEmptyScope(parent)

	scope.Def("*dir*",
		state.Dir,
		`working directory`,
		`This value is always set to the directory containing the file being run.`,
		`It can and should be used to load sibling/child paths, e.g. *dir*/foo to load the 'foo.bass' file in the same directory as the current file.`)

	scope.Def("*args*",
		state.Args,
		`command line arguments`)

	scope.Def("*stdin*",
		state.Stdin,
		`standard input stream`)

	scope.Def("*stdout*",
		state.Stdout,
		`standard output sink`)

	return bass.NewEmptyScope(scope)
}
