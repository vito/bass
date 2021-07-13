package bass

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
)

type Path interface {
	Value

	// Resolve returns the absolute path on the local filesystem relative to the
	// given path.
	Resolve(string) (string, error)

	// DirectoryPath extends the path and returns either a DirectoryPath or a
	// FilePath.
	//
	// FilePath returns an error; it shouldn't be possible to extend a file path,
	// and this is most likely an accident.
	//
	// CommandPath also returns an error; extending a command doesn't make sense,
	// and this is most likely an accident.
	Extend(Path) (Path, error)
}

// ., ./foo/
type DirectoryPath struct {
	Path string
}

var _ Value = DirectoryPath{}

func (value DirectoryPath) String() string {
	return value.Path + "/"
}

func (value DirectoryPath) Equal(other Value) bool {
	var o DirectoryPath
	return other.Decode(&o) == nil && value.Path == o.Path
}

func (value DirectoryPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *DirectoryPath:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value DirectoryPath) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Path = DirectoryPath{}

func (dir DirectoryPath) Resolve(root string) (string, error) {
	return filepath.FromSlash(path.Join(root, dir.Path)), nil
}

func (dir DirectoryPath) Extend(ext Path) (Path, error) {
	switch p := ext.(type) {
	case DirectoryPath:
		return DirectoryPath{
			Path: dir.Path + "/" + p.Path,
		}, nil
	case FilePath:
		return FilePath{
			Path: dir.Path + "/" + p.Path,
		}, nil
	default:
		return nil, fmt.Errorf("impossible: extending path with %T", p)
	}
}

// ./foo
type FilePath struct {
	Path string
}

var _ Value = FilePath{}

func (value FilePath) String() string {
	return value.Path
}

func (value FilePath) Equal(other Value) bool {
	var o FilePath
	return other.Decode(&o) == nil && value.Path == o.Path
}

func (value FilePath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *FilePath:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value FilePath) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Combiner = FilePath{}

func (combiner FilePath) Call(val Value, env *Env, cont Cont) ReadyCont {
	return makeNativeWorkload(val, env, cont, combiner)
}

var _ Path = FilePath{}

func (path_ FilePath) Resolve(root string) (string, error) {
	return filepath.FromSlash(path.Join(root, path_.Path)), nil
}

func (path_ FilePath) Extend(ext Path) (Path, error) {
	// TODO: better error
	return nil, fmt.Errorf("cannot extend file path: %s", path_.Path)
}

// .foo
type CommandPath struct {
	Command string
}

var _ Value = CommandPath{}

func (value CommandPath) String() string {
	return "." + value.Command
}

func (value CommandPath) Equal(other Value) bool {
	var o CommandPath
	return other.Decode(&o) == nil && value.Command == o.Command
}

func (value CommandPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *CommandPath:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value CommandPath) Eval(env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Path = CommandPath{}

func (path CommandPath) Resolve(root string) (string, error) {
	return exec.LookPath(path.Command)
}

var _ Combiner = CommandPath{}

func (combiner CommandPath) Call(val Value, env *Env, cont Cont) ReadyCont {
	return makeNativeWorkload(val, env, cont, combiner)
}

var _ Path = CommandPath{}

func (path CommandPath) Extend(ext Path) (Path, error) {
	// TODO: better error
	return nil, fmt.Errorf("cannot extend command path: %s", path.Command)
}

type ExtendPath struct {
	Parent Value
	Child  Path
}

var _ Value = ExtendPath{}

func (value ExtendPath) String() string {
	return fmt.Sprintf("%s%s", value.Parent, value.Child)
}

func (value ExtendPath) Equal(other Value) bool {
	var o ExtendPath
	if err := other.Decode(&o); err != nil {
		return false
	}

	return value.Parent.Equal(o.Parent) && value.Child.Equal(o.Child)
}

func (value ExtendPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *ExtendPath:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

// Eval returns the value.
func (value ExtendPath) Eval(env *Env, cont Cont) ReadyCont {
	return value.Parent.Eval(env, Continue(func(parent Value) Value {
		var path Path
		if err := parent.Decode(&path); err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(path.Extend(value.Child))
	}))
}

func makeNativeWorkload(val Value, env *Env, cont Cont, path_ Path) ReadyCont {
	var list List
	err := val.Decode(&list)
	if err != nil {
		return cont.Call(nil, fmt.Errorf("call path: %w", err))
	}

	return ToCons(list).Eval(env, Continue(func(args Value) Value {
		cmd, err := ValueOf(NativeCommand{
			Path:  path_,
			Stdin: args,
		})
		if err != nil {
			return cont.Call(nil, fmt.Errorf("call path: %w", err))
		}

		return cont.Call(ValueOf(Workload{
			Platform: NativePlatform,
			Command:  cmd,
		}))
	}))
}
