package bass

const (
	RunBindingStdin  Symbol = "*stdin*"
	RunBindingStdout Symbol = "*stdout*"
	RunBindingDir    Symbol = "*dir*"
	RunBindingEnv    Symbol = "*env*"
	RunBindingMain   Symbol = "main"
)

type RunState struct {
	Dir    Path
	Env    *Scope
	Stdin  *Source
	Stdout *Sink
}

func NewRunScope(parent *Scope, state RunState) *Scope {
	scope := NewEmptyScope(parent)

	dir := state.Dir
	if dir == nil {
		dir = DirPath{Path: "."}
	}

	var env *Scope
	if state.Env == nil {
		env = NewEmptyScope()
	} else {
		env = state.Env.Copy()
	}

	stdin := state.Stdin
	if stdin == nil {
		stdin = NewSource(NewInMemorySource())
	}

	stdout := state.Stdout
	if stdout == nil {
		stdout = NewSink(NewInMemorySink())
	}

	scope.Set(RunBindingDir, dir, `current working directory`,
		`This value is always set to the directory containing the script being run.`,
		`It can and should be used to load sibling/child paths, e.g. *dir*/foo to load the 'foo.bass' file in the same directory as the current file.`)

	scope.Set(RunBindingEnv, env, `environment variables`,
		`System environment variables are only available to the entrypoint script. To propagate them further they must be explicitly passed to thunks using [with-env].`,
		`System environment variables are unset from the physical OS process as part of initialization to ensure they cannot be leaked.`)

	scope.Set(RunBindingStdin, stdin, `standard input stream`,
		`Values read from *stdin* will be parsed from the process's stdin as a JSON stream.`)

	scope.Set(RunBindingStdout, stdout, `standard output sink`,
		`Values emitted by a script to *stdout* will be encoded as a JSON stream to the process's stdout.`)

	scope.Set(RunBindingMain, Func("main", "[]", func() {}),
		`script entrypoint`,
		`The [script:main] function is called with any provided command-line args when running a Bass script.`,
		`Scripts should define it to capture system arguments and run the script's desired effects.`,
		`Putting effects in [script:main] instead of running them at the toplevel makes the Bass language server happier.`)

	return NewEmptyScope(scope)
}
