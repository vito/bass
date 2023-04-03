package bass

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/vito/bass/pkg/proto"
)

// Path is an abstract location identifier for files, directories, or
// executable commands.
type Path interface {
	// All Paths are Values.
	Value

	// Name returns the unqualified name for the path, i.e. the base name of a
	// file or directory, or the name of a command.
	Name() string

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

func clarifyPath(p string) string {
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") {
		return p
	}

	return "./" + p
}

func (value DirPath) String() string {
	return value.Slash()
}

func (value DirPath) Slash() string {
	return clarifyPath(value.Path + "/")
}

func (value DirPath) Equal(other Value) bool {
	var o DirPath
	return other.Decode(&o) == nil && value.Path == o.Path
}

func (value DirPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *Path:
		*x = value
		return nil
	case *Applicative:
		*x = value
		return nil
	case *Combiner:
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

func (path *DirPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.DirPath)
	if !ok {
		return DecodeError{msg, path}
	}

	path.Path = p.Path

	return nil
}

// Eval returns the value.
func (value DirPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = DirPath{}

func (combiner DirPath) Unwrap() Combiner {
	return ExtendOperative{combiner}
}

var _ Combiner = DirPath{}

func (combiner DirPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = DirPath{}

func (value DirPath) Name() string {
	return path.Base(value.Path)
}

func (dir DirPath) Extend(ext Path) (Path, error) {
	switch p := ext.(type) {
	case DirPath:
		return DirPath{
			Path: path.Clean(dir.Path + "/" + p.Path),
		}, nil
	case FilePath:
		return FilePath{
			Path: path.Clean(dir.Path + "/" + p.Path),
		}, nil
	default:
		return nil, ExtendError{dir, ext}
	}
}

var _ FilesystemPath = DirPath{}

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

func (value DirPath) Dir() DirPath {
	return DirPath{path.Dir(value.Path)}
}

func (value DirPath) IsDir() bool {
	return true
}

var _ Bindable = DirPath{}

func (binding DirPath) Bind(_ context.Context, _ *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	return cont.Call(binding, BindConst(binding, val))
}

func (DirPath) EachBinding(func(Symbol, Range) error) error {
	return nil
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
	return value.Slash()
}

func (value FilePath) Slash() string {
	return clarifyPath(value.Path)
}

func (value FilePath) Equal(other Value) bool {
	var o FilePath
	return other.Decode(&o) == nil && value.Path == o.Path
}

func (value FilePath) Decode(dest any) error {
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

func (path *FilePath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.FilePath)
	if !ok {
		return DecodeError{msg, path}
	}

	path.Path = p.Path

	return nil
}

// Eval returns the value.
func (value FilePath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = FilePath{}

func (app FilePath) Unwrap() Combiner {
	return ThunkOperative{
		Cmd: app,
	}
}

func (app FilePath) FileOrDir() FileOrDirPath {
	return FileOrDirPath{File: &app}
}

var _ Combiner = FilePath{}

func (combiner FilePath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = FilePath{}

func (value FilePath) Name() string {
	return path.Base(value.Path)
}

func (path_ FilePath) Extend(ext Path) (Path, error) {
	return nil, ExtendError{path_, ext}
}

var _ FilesystemPath = FilePath{}

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

var _ Bindable = FilePath{}

func (binding FilePath) Bind(_ context.Context, _ *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	return cont.Call(binding, BindConst(binding, val))
}

func (FilePath) EachBinding(func(Symbol, Range) error) error {
	return nil
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

func (value CommandPath) Decode(dest any) error {
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

func (path *CommandPath) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.CommandPath)
	if !ok {
		return DecodeError{msg, path}
	}

	path.Command = p.Name

	return nil
}

// Eval returns the value.
func (value CommandPath) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = CommandPath{}

func (app CommandPath) Unwrap() Combiner {
	return ThunkOperative{
		Cmd: app,
	}
}

var _ Combiner = CommandPath{}

func (combiner CommandPath) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

var _ Path = CommandPath{}

func (value CommandPath) Name() string {
	return value.Command
}

func (path CommandPath) Extend(ext Path) (Path, error) {
	return nil, ExtendError{path, ext}
}

var _ Bindable = CommandPath{}

func (binding CommandPath) Bind(_ context.Context, _ *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	return cont.Call(binding, BindConst(binding, val))
}

func (CommandPath) EachBinding(func(Symbol, Range) error) error {
	return nil
}

// ExtendPath extends a parent path expression with a child path.
type ExtendPath struct {
	Parent Value
	Child  FilesystemPath
}

var _ Value = ExtendPath{}

func (value ExtendPath) String() string {
	sub := path.Clean(value.Child.String())
	if value.Child.IsDir() {
		sub += "/"
	}

	switch value.Parent.(type) {
	case Path, ExtendPath:
		return fmt.Sprintf("%s%s", value.Parent, sub)
	default:
		return fmt.Sprintf("%s/%s", value.Parent, sub)
	}
}

func (value ExtendPath) Equal(other Value) bool {
	var o ExtendPath
	if err := other.Decode(&o); err != nil {
		return false
	}

	return value.Parent.Equal(o.Parent) && value.Child.Equal(o.Child)
}

func (value ExtendPath) Decode(dest any) error {
	switch x := dest.(type) {
	case *Bindable:
		*x = value
		return nil
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
		var comb Combiner
		if err := parent.Decode(&comb); err != nil {
			return cont.Call(nil, fmt.Errorf("subpath %s: %w", parent, err))
		}

		return comb.Call(ctx, NewList(value.Child), scope, cont)
	}))
}

var _ Bindable = ExtendPath{}

func (binding ExtendPath) Bind(ctx context.Context, scope *Scope, cont Cont, val Value, _ ...Annotated) ReadyCont {
	return binding.Eval(ctx, scope, Continue(func(res Value) Value {
		return cont.Call(binding, BindConst(res, val))
	}))
}

func (ExtendPath) EachBinding(func(Symbol, Range) error) error {
	return nil
}

// ThunkOperative is an operative which constructs a Thunk.
type ThunkOperative struct {
	Cmd Value
}

var _ Value = ThunkOperative{}

func (value ThunkOperative) String() string {
	return fmt.Sprintf("(unwrap %s)", value.Cmd)
}

func (value ThunkOperative) Equal(other Value) bool {
	var o ThunkOperative
	return other.Decode(&o) == nil && value == o
}

func (value ThunkOperative) Decode(dest any) error {
	switch x := dest.(type) {
	case *ThunkOperative:
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

func (value ThunkOperative) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// Call constructs a thunk, passing arguments as values on stdin.
func (op ThunkOperative) Call(_ context.Context, args Value, _ *Scope, cont Cont) ReadyCont {
	var stdin []Value
	if err := args.Decode(&stdin); err != nil {
		return cont.Call(nil, err)
	}

	if len(stdin) == 0 {
		stdin = nil
	}

	return cont.Call(Thunk{
		Args:  []Value{op.Cmd},
		Stdin: stdin,
	}, nil)
}

// ExtendOperative is an operative which constructs a Extend.
type ExtendOperative struct {
	Path Path
}

var _ Value = ExtendOperative{}

func (value ExtendOperative) String() string {
	return fmt.Sprintf("(unwrap %s)", value.Path)
}

func (value ExtendOperative) Equal(other Value) bool {
	var o ExtendOperative
	return other.Decode(&o) == nil && value == o
}

func (value ExtendOperative) Decode(dest any) error {
	switch x := dest.(type) {
	case *ExtendOperative:
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

func (value ExtendOperative) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

// Call constructs a thunk, passing arguments as values on stdin.
func (op ExtendOperative) Call(_ context.Context, val Value, _ *Scope, cont Cont) ReadyCont {

	var args []Value
	if err := val.Decode(&args); err != nil {
		return cont.Call(nil, err)
	}

	if len(args) != 1 {
		return cont.Call(nil, ArityError{
			Need: 1,
			Have: len(args),
		})
	}

	var sub Path
	if err := args[0].Decode(&sub); err != nil {
		return cont.Call(nil, err)
	}

	return cont.Call(op.Path.Extend(sub))
}
