package bass

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
)

// Path is an abstract location identifier for files, directories, or
// executable commands.
type Path interface {
	// All Paths are Values.
	Value

	// Extend returns a path referring to the given path relative to the parent
	// Path.
	Extend(Path) (Path, error)
}

// DirPath represents a directory path in an abstract filesystem.
//
// Its interpretation is context-dependent; it may refer to a path in a runtime
// environment, or a path on the local machine.
type DirPath struct {
	Path string `json:"dir"`
}

var _ Value = DirPath{}

func (value DirPath) String() string {
	return value.Path + "/"
}

func (value DirPath) FromSlash() string {
	fs := filepath.Clean(filepath.FromSlash(value.Path))

	if filepath.IsAbs(fs) {
		return fs + string(filepath.Separator)
	} else if fs == "." {
		return fs + string(filepath.Separator)
	} else {
		return "." + string(filepath.Separator) + fs + string(filepath.Separator)
	}
}

func (value DirPath) IsDir() bool {
	return true
}

func (value DirPath) Equal(other Value) bool {
	var o DirPath
	return other.Decode(&o) == nil && value.Path == o.Path
}

func (value DirPath) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Path:
		*x = value
		return nil
	case *Value:
		*x = value
		return nil
	case *DirPath:
		*x = value
		return nil
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *DirPath) FromValue(val Value) error {
	var scope *Scope
	if err := val.Decode(&scope); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(scope, value)
}

// Eval returns the value.
func (value DirPath) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Path = DirPath{}
var _ FilesystemPath = DirPath{}

func (dir DirPath) Extend(ext Path) (Path, error) {
	switch p := ext.(type) {
	case DirPath:
		return DirPath{
			Path: dir.Path + "/" + p.Path,
		}, nil
	case FilePath:
		return FilePath{
			Path: dir.Path + "/" + p.Path,
		}, nil
	default:
		return nil, ExtendError{dir, ext}
	}
}

// FilePath represents a file path in an abstract filesystem.
//
// Its interpretation is context-dependent; it may refer to a path in a runtime
// environment, or a path on the local machine.
type FilePath struct {
	Path string `json:"file"`
}

var _ Value = FilePath{}

func (value FilePath) String() string {
	return value.Path
}

func (value FilePath) FromSlash() string {
	fs := filepath.Clean(filepath.FromSlash(value.Path))

	if filepath.IsAbs(fs) {
		return fs
	} else {
		return "." + string(filepath.Separator) + fs
	}
}

func (value FilePath) Dir() DirPath {
	return DirPath{path.Dir(value.Path)}
}

func (value FilePath) IsDir() bool {
	return false
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
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *FilePath) FromValue(val Value) error {
	var scope *Scope
	if err := val.Decode(&scope); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(scope, value)
}

// Eval returns the value.
func (value FilePath) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = FilePath{}

func (app FilePath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = FilePath{}

func (combiner FilePath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(PathOperative{combiner}).Call(ctx, val, scope, cont)
}

var _ Path = FilePath{}
var _ FilesystemPath = FilePath{}

func (path_ FilePath) Extend(ext Path) (Path, error) {
	return nil, ExtendError{path_, ext}
}

// CommandPath represents a command path in an abstract runtime environment,
// typically resolved by consulting $PATH.
type CommandPath struct {
	Command string `json:"command"`
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
	case *Bindable:
		*x = value
		return nil
	case Decodable:
		return x.FromValue(value)
	default:
		return DecodeError{
			Source:      value,
			Destination: dest,
		}
	}
}

func (value *CommandPath) FromValue(val Value) error {
	var scope *Scope
	if err := val.Decode(&scope); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(scope, value)
}

// Eval returns the value.
func (value CommandPath) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Path = CommandPath{}

var _ Applicative = CommandPath{}

func (app CommandPath) Unwrap() Combiner {
	return PathOperative{app}
}

var _ Combiner = CommandPath{}

func (combiner CommandPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(PathOperative{combiner}).Call(ctx, val, scope, cont)
}

var _ Path = CommandPath{}

func (path CommandPath) Extend(ext Path) (Path, error) {
	return nil, ExtendError{path, ext}
}

var _ Bindable = CommandPath{}

func (binding CommandPath) Bind(scope *Scope, val Value) error {
	return BindConst(binding, val)
}

// ExtendPath extends a parent path expression with a child path.
type ExtendPath struct {
	Parent Value
	Child  Path
}

var _ Value = ExtendPath{}

func (value ExtendPath) String() string {
	switch value.Parent.(type) {
	case Path, ExtendPath:
		return fmt.Sprintf("%s%s", value.Parent, value.Child)
	default:
		return fmt.Sprintf("%s/%s", value.Parent, value.Child)
	}
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

// Eval evaluates the Parent value into a Path and extends it with Child.
func (value ExtendPath) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return value.Parent.Eval(ctx, scope, Continue(func(parent Value) Value {
		var path Path
		if err := parent.Decode(&path); err != nil {
			return cont.Call(nil, err)
		}

		return cont.Call(path.Extend(value.Child))
	}))
}

// PathOperative is an operative which constructs a Workload.
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

func (value PathOperative) Eval(ctx context.Context, scope *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// Call constructs a Workload, interpreting keyword arguments as fields and
// regular arguments as values for the Stdin field.
func (op PathOperative) Call(ctx context.Context, args Value, scope *Scope, cont Cont) ReadyCont {
	kwargs := Bindings{
		"path":  op.Path,
		"stdin": args,
		"response": Bindings{
			"stdout": Bool(true),
		}.Scope(),
	}.Scope()

	var workload Workload
	if err := kwargs.Decode(&workload); err != nil {
		return cont.Call(nil, err)
	}

	return cont.Call(ValueOf(workload))
}
