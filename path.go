package bass

import (
	"context"
	"fmt"
	"os/exec"
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
	Path string `bass:"dir" json:"dir"`
}

var _ Value = DirectoryPath{}

func (value DirectoryPath) String() string {
	return value.Path + string(filepath.Separator)
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

func (value *DirectoryPath) FromObject(obj Object) error {
	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value DirectoryPath) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Path = DirectoryPath{}

func (dir DirectoryPath) Resolve(root string) (string, error) {
	if filepath.IsAbs(dir.Path) {
		return dir.Path, nil
	}

	return filepath.Join(root, dir.Path), nil
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
	Path string `bass:"file" json:"file"`
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
	case *Applicative:
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

func (value *FilePath) FromObject(obj Object) error {
	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value FilePath) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = FilePath{}

func (app FilePath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = FilePath{}

func (combiner FilePath) Call(ctx context.Context, val Value, env *Env, cont Cont) ReadyCont {
	return Wrapped{PathOperative{combiner}}.Call(ctx, val, env, cont)
}

var _ Path = FilePath{}

func (file FilePath) Resolve(root string) (string, error) {
	if filepath.IsAbs(file.Path) {
		return file.Path, nil
	}

	return filepath.Join(root, file.Path), nil
}

func (path_ FilePath) Extend(ext Path) (Path, error) {
	// TODO: better error
	return nil, fmt.Errorf("cannot extend file path: %s", path_.Path)
}

// .foo
type CommandPath struct {
	Command string `bass:"command" json:"command"`
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
	case *Applicative:
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

func (value *CommandPath) FromObject(obj Object) error {
	return decodeStruct(obj, value)
}

// Eval returns the value.
func (value CommandPath) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Path = CommandPath{}

func (path CommandPath) Resolve(root string) (string, error) {
	return exec.LookPath(path.Command)
}

var _ Applicative = CommandPath{}

func (app CommandPath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = CommandPath{}

func (combiner CommandPath) Call(ctx context.Context, val Value, env *Env, cont Cont) ReadyCont {
	return Wrapped{PathOperative{combiner}}.Call(ctx, val, env, cont)
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
func (value ExtendPath) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return value.Parent.Eval(ctx, env, Continue(func(parent Value) Value {
		var path Path
		if err := parent.Decode(&path); err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(path.Extend(value.Child))
	}))
}

type PathOperative struct {
	Path Path
}

var _ Value = PathOperative{}

func (value PathOperative) String() string {
	return fmt.Sprintf("(unwrap %s)", value.Path)
}

func (value PathOperative) Equal(other Value) bool {
	var o PathOperative
	return other.Decode(&o) == nil && value == o
}

func (value PathOperative) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *PathOperative:
		*x = value
		return nil
	case *Combiner:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value PathOperative) Eval(ctx context.Context, env *Env, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

func (op PathOperative) Call(ctx context.Context, args Value, env *Env, cont Cont) ReadyCont {
	command := Object{
		"path": op.Path,
	}

	stdin := []Value{}
	var kw Keyword
	err := Each(args.(List), func(val Value) error {
		if err := val.Decode(&kw); err == nil {
			return nil
		}

		if kw != "" {
			command[kw] = val
			kw = ""
			return nil
		}

		stdin = append(stdin, val)
		return nil
	})
	if err != nil {
		return cont.Call(nil, err)
	}

	if len(stdin) > 0 {
		command["stdin"] = NewList(stdin...)
	}

	var check NativeCommand
	if err := command.Decode(&check); err != nil {
		return cont.Call(nil, err)
	}

	return cont.Call(ValueOf(Workload{
		Platform: NativePlatform,
		Command:  command,
	}))
}
