package bass

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"math/rand"
	"reflect"
	"strings"

	"github.com/vito/invaders"
)

type Thunk struct {
	// Image specifies the OCI image in which to run the thunk.
	Image *ThunkImage `json:"image,omitempty"`

	// Insecure may be set to true to enable running the thunk with elevated
	// privileges. Its meaning is determined by the runtime.
	Insecure bool `json:"insecure,omitempty"`

	// Cmd identifies the file or command to run.
	Cmd ThunkCmd `json:"cmd"`

	// Args is a list of string or path arguments to pass to the command.
	Args []Value `json:"args,omitempty"`

	// Stdin is a list of arbitrary values, which may contain paths, to pass to
	// the command.
	Stdin []Value `json:"stdin,omitempty"`

	// Env is a mapping from environment variables to their string or path
	// values.
	Env *Scope `json:"env,omitempty"`

	// Dir configures a working directory in which to run the command.
	//
	// Note that a working directory is automatically provided to thunks by
	// the runtime. A relative Dir value will be relative to this working
	// directory, not the OCI image's initial working directory. The OCI image's
	// working directory is ignored.
	//
	// A relative directory path will be relative to the initial working
	// directory. An absolute path will be relative to the OCI image root.
	//
	// A thunk directory path may also be provided. It will be mounted to the
	// container and used as the working directory of the command.
	Dir *ThunkDir `json:"dir,omitempty"`

	// Mounts configures explicit mount points for the thunk, in addition to
	// any provided in Path, Args, Stdin, Env, or Dir.
	Mounts []ThunkMount `json:"mounts,omitempty"`

	// Labels specify arbitrary fields for identifying the thunk, typically
	// used to influence caching behavior.
	//
	// For example, thunks which may return different results over time should
	// embed the current timestamp truncated to a certain amount of granularity,
	// e.g. one minute. Doing so prevents the first call from being cached
	// forever while still allowing some level of caching to take place.
	Labels *Scope `json:"labels,omitempty"`
}

func MustThunk(cmd Path, stdin ...Value) Thunk {
	var thunkCmd ThunkCmd
	if err := cmd.Decode(&thunkCmd); err != nil {
		panic(fmt.Sprintf("MustParse: %s", err))
	}

	return Thunk{
		Cmd:   thunkCmd,
		Stdin: stdin,
	}
}

func (thunk Thunk) Cmdline() string {
	var cmdline []string

	cmdPath := thunk.Cmd.ToValue()
	var cmd CommandPath
	if err := cmdPath.Decode(&cmd); err == nil {
		cmdline = append(cmdline, cmd.Name())
	} else {
		cmdline = append(cmdline, cmdPath.String())
	}

	for _, arg := range thunk.Args {
		var str string
		if err := arg.Decode(&str); err == nil && !strings.Contains(str, " ") {
			cmdline = append(cmdline, str)
		} else {
			cmdline = append(cmdline, arg.String())
		}
	}

	return strings.Join(cmdline, " ")
}

// WithImage sets the base image of the thunk, recursing into parent thunks until
// it reaches the bottom, like a rebase.
func (thunk Thunk) WithImage(image ThunkImage) Thunk {
	if thunk.Image != nil && thunk.Image.Thunk != nil {
		rebased := thunk.Image.Thunk.WithImage(image)
		thunk.Image = &ThunkImage{
			Thunk: &rebased,
		}
		return thunk
	}

	thunk.Image = &image
	return thunk
}

// WithArgs sets the thunk's arg values.
func (thunk Thunk) WithArgs(args ...Value) Thunk {
	thunk.Args = args
	return thunk
}

// WithEnv sets the thunk's env.
func (thunk Thunk) WithEnv(env *Scope) Thunk {
	thunk.Env = env
	return thunk
}

// WithStdin sets the thunk's stdin values.
func (thunk Thunk) WithStdin(stdin ...Value) Thunk {
	thunk.Stdin = stdin
	return thunk
}

// WithInsecure sets whether the thunk should be run insecurely.
func (thunk Thunk) WithInsecure(insecure bool) Thunk {
	thunk.Insecure = insecure
	return thunk
}

// WithDir sets the thunk's working directory.
func (thunk Thunk) WithDir(dir ThunkDir) Thunk {
	thunk.Dir = &dir
	return thunk
}

// WithMount adds a mount.
func (thunk Thunk) WithMount(src ThunkMountSource, tgt FileOrDirPath) Thunk {
	thunk.Mounts = append(thunk.Mounts, ThunkMount{
		Source: src,
		Target: tgt,
	})
	return thunk
}

// WithMount adds a mount.
func (thunk Thunk) WithLabel(key Symbol, val Value) Thunk {
	if thunk.Labels == nil {
		thunk.Labels = NewEmptyScope()
	}

	thunk.Labels = thunk.Labels.Copy()
	thunk.Labels.Set(key, val)
	return thunk
}

// Wrap wraps the thunk's cmd + args with an outer cmd and args.
func (thunk Thunk) Wrap(cmd ThunkCmd, prependArgs ...Value) Thunk {
	thunk.Args = append(append([]Value{thunk.Cmd.ToValue()}, prependArgs...), thunk.Args...)
	thunk.Cmd = cmd
	return thunk
}

var _ Value = Thunk{}

func (thunk Thunk) String() string {
	return fmt.Sprintf("<thunk: %s name:%s>", NewList(thunk.Cmd.ToValue()), thunk.Name())
}

func (thunk Thunk) Equal(other Value) bool {
	// TODO: this is lazy, but the comparison would be insanely complicated and
	// error prone to implement with very little benefit. and i'd rather not
	// marshal here and risk encountering an err.
	//
	// maybe consider cmp package? i forget if it's able to use Equal
	return reflect.DeepEqual(thunk, other)
}

var _ Path = Thunk{}

// Name returns the unqualified name for the path, i.e. the base name of a
// file or directory, or the name of a command.
func (thunk Thunk) Name() string {
	digest, err := thunk.SHA1()
	if err != nil {
		// this is awkward, but it's better than panicking
		return fmt.Sprintf("(error: %s)", err)
	}

	return digest
}

// Extend returns a path referring to the given path relative to the parent
// Path.
func (thunk Thunk) Extend(sub Path) (Path, error) {
	return ThunkPath{
		Thunk: thunk,
		Path:  FileOrDirPath{Dir: &DirPath{"."}},
	}.Extend(sub)
}

func (thunk Thunk) Decode(dest interface{}) error {
	switch x := dest.(type) {
	case *Thunk:
		*x = thunk
		return nil
	case *Path:
		*x = thunk
		return nil
	case *Value:
		*x = thunk
		return nil
	case *Combiner:
		*x = thunk
		return nil
	case *Readable:
		*x = thunk
		return nil
	case Decodable:
		return x.FromValue(thunk)
	default:
		return DecodeError{
			Source:      thunk,
			Destination: dest,
		}
	}
}

func (value *Thunk) FromValue(val Value) error {
	var scope *Scope
	if err := val.Decode(&scope); err != nil {
		return fmt.Errorf("%T.FromValue: %w", value, err)
	}

	return decodeStruct(scope, value)
}

// Eval returns the thunk.
func (value Thunk) Eval(_ context.Context, _ *Scope, cont Cont) ReadyCont {
	return cont.Call(value, nil)
}

var _ Applicative = Thunk{}

func (combiner Thunk) Unwrap() Combiner {
	return ExtendOperative{
		ThunkPath{
			Thunk: combiner,
			Path: FileOrDirPath{
				Dir: &DirPath{"."},
			},
		},
	}
}

var _ Combiner = Thunk{}

func (combiner Thunk) Call(ctx context.Context, val Value, scope *Scope, cont Cont) ReadyCont {
	return Wrap(combiner.Unwrap()).Call(ctx, val, scope, cont)
}

func (thunk *Thunk) UnmarshalJSON(b []byte) error {
	var obj *Scope
	err := json.Unmarshal(b, &obj)
	if err != nil {
		return err
	}

	return obj.Decode(thunk)
}

func (thunk *Thunk) Platform() *Platform {
	if thunk.Image == nil {
		return nil
	}

	return thunk.Image.Platform()
}

// SHA1 returns a stable SHA1 hash derived from the thunk.
func (wl Thunk) SHA1() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha1.Sum(payload)), nil
}

// SHA256 returns a stable SHA256 hash derived from the thunk.
func (wl Thunk) SHA256() (string, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

// Avatar returns an ASCII art avatar derived from the thunk.
func (wl Thunk) Avatar() (*invaders.Invader, error) {
	payload, err := json.Marshal(wl)
	if err != nil {
		return nil, err
	}

	h := fnv.New64a()
	_, err = h.Write(payload)
	if err != nil {
		return nil, err
	}

	invader := &invaders.Invader{}
	invader.Set(rand.New(rand.NewSource(int64(h.Sum64()))))
	return invader, nil
}

var _ Readable = Thunk{}

func (thunk Thunk) Open(ctx context.Context) (io.ReadCloser, error) {
	pool, err := RuntimeFromContext(ctx, thunk.Platform())
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()
	go func() {
		w.CloseWithError(pool.Run(ctx, w, thunk))
	}()

	return r, nil
}
