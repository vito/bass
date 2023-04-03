package bass

import (
	"context"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"

	"github.com/vito/bass/pkg/proto"
	"github.com/vito/bass/std"
	"github.com/vito/invaders"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/encoding/protojson"
	gproto "google.golang.org/protobuf/proto"
)

type Thunk struct {
	// Image specifies the OCI image in which to run the thunk.
	Image *ThunkImage `json:"image,omitempty"`

	// Insecure may be set to true to enable running the thunk with elevated
	// privileges. Its meaning is determined by the runtime.
	Insecure bool `json:"insecure,omitempty"`

	// Args is a list of string or path arguments.
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

	// Ports is a mapping from arbitrary names to port numbers on which the
	// command listens.
	//
	// Ports may be referenced by ThunkAddrs. When a ThunkAddr is used by another
	// thunk its embedded thunk will be started and all ports will be polled
	// until they are listening.
	Ports []ThunkPort `json:"ports,omitempty"`

	// TLS configures paths to place generated certificates.
	TLS *ThunkTLS `json:"tls,omitempty"`

	// Entrypoint configures a static command and arguments that will be
	// prepended to child commands.
	//
	// A nil value inherits from the parent. An empty slice removes it.
	Entrypoint []string `json:"entrypoint,omitempty"`

	// DefaultArgs configures a command and arguments to use if a child execution
	// does not specify a command.
	DefaultArgs []string `json:"default_args,omitempty"`
}

type ThunkPort struct {
	Name string `json:"name"`
	Port int    `json:"port"`
}

type ThunkTLS struct {
	Cert FilePath `json:"cert"`
	Key  FilePath `json:"key"`
}

func (thunk *Thunk) UnmarshalProto(msg proto.Message) error {
	p, ok := msg.(*proto.Thunk)
	if !ok {
		return fmt.Errorf("unmarshal proto: %w", DecodeError{msg, thunk})
	}

	if p.Image != nil {
		thunk.Image = &ThunkImage{}
		if err := thunk.Image.UnmarshalProto(p.Image); err != nil {
			return err
		}
	}

	thunk.Insecure = p.Insecure

	for i, arg := range p.Args {
		val, err := FromProto(arg)
		if err != nil {
			return fmt.Errorf("unmarshal proto arg[%d]: %w", i, err)
		}

		thunk.Args = append(thunk.Args, val)
	}

	for i, stdin := range p.Stdin {
		val, err := FromProto(stdin)
		if err != nil {
			return fmt.Errorf("unmarshal proto stdin[%d]: %w", i, err)
		}

		thunk.Stdin = append(thunk.Stdin, val)
	}

	if len(p.Env) > 0 {
		thunk.Env = NewEmptyScope()

		for _, bnd := range p.Env {
			val, err := FromProto(bnd.Value)
			if err != nil {
				return fmt.Errorf("unmarshal proto env[%s]: %w", bnd.Symbol, err)
			}

			thunk.Env.Set(Symbol(bnd.Symbol), val)
		}
	}

	if p.Dir != nil {
		thunk.Dir = &ThunkDir{}
		if err := thunk.Dir.UnmarshalProto(p.Dir); err != nil {
			return fmt.Errorf("unmarshal proto dir: %w", err)
		}
	}

	for i, mount := range p.Mounts {
		var mnt ThunkMount
		if err := mnt.UnmarshalProto(mount); err != nil {
			return fmt.Errorf("unmarshal proto mount[%d]: %w", i, err)
		}

		thunk.Mounts = append(thunk.Mounts, mnt)
	}

	if len(p.Labels) > 0 {
		thunk.Labels = NewEmptyScope()

		for _, bnd := range p.Labels {
			val, err := FromProto(bnd.Value)
			if err != nil {
				return fmt.Errorf("unmarshal proto label[%s]: %w", bnd.Symbol, err)
			}

			thunk.Labels.Set(Symbol(bnd.Symbol), val)
		}
	}

	if len(p.Ports) > 0 {
		for _, port := range p.Ports {
			thunk.Ports = append(thunk.Ports, ThunkPort{
				Name: port.GetName(),
				Port: int(port.GetPort()),
			})
		}
	}

	if p.Tls != nil {
		thunk.TLS = &ThunkTLS{}
		err := thunk.TLS.Cert.UnmarshalProto(p.Tls.Cert)
		if err != nil {
			return fmt.Errorf("unmarshal cert: %w", err)
		}

		err = thunk.TLS.Key.UnmarshalProto(p.Tls.Key)
		if err != nil {
			return fmt.Errorf("unmarshal key: %w", err)
		}
	}

	return nil
}

func MustThunk(cmd Path, stdin ...Value) Thunk {
	return Thunk{
		Args:  []Value{cmd},
		Stdin: stdin,
	}
}

func (thunk Thunk) Run(ctx context.Context) error {
	platform := thunk.Platform()

	if platform != nil {
		runtime, err := RuntimeFromContext(ctx, *platform)
		if err != nil {
			return err
		}

		return runtime.Run(ctx, thunk)
	} else {
		return Bass.Run(ctx, thunk, thunk.RunState(io.Discard))
	}
}

func (thunk Thunk) RunState(stdout io.Writer) RunState {
	return RunState{
		Dir:    thunk.RunDir(),
		Env:    thunk.Env,
		Stdin:  NewSource(NewInMemorySource(thunk.Stdin...)),
		Stdout: NewSink(NewJSONSink(thunk.String(), stdout)),
	}
}

func (thunk Thunk) RunDir() Path {
	if len(thunk.Args) == 0 {
		panic(fmt.Sprintf("Thunk.RunDir: no arguments: %+v", thunk))
	}

	cmd := thunk.Args[0]

	var filep FilePath
	var thunkp ThunkPath
	var cmdp CommandPath
	var hostp HostPath
	var fsp *FSPath
	var cachep CachePath
	if cmd.Decode(&filep) == nil {
		return filep.Dir()
	} else if cmd.Decode(&thunkp) == nil {
		return thunkp.Dir()
	} else if cmd.Decode(&cmdp) == nil {
		return NewFSDir(std.FS)
	} else if cmd.Decode(&hostp) == nil {
		return hostp.Dir()
	} else if cmd.Decode(&fsp) == nil {
		return fsp.Dir()
	} else if cmd.Decode(&cachep) == nil {
		return cachep.Dir()
	} else {
		panic(fmt.Sprintf("ThunkCmd.RunDir: no value present: %+v", cmd))
	}
}

func (thunk Thunk) Read(ctx context.Context, w io.Writer) error {
	platform := thunk.Platform()

	if platform != nil {
		runtime, err := RuntimeFromContext(ctx, *platform)
		if err != nil {
			return err
		}

		return runtime.Read(ctx, w, thunk)
	} else {
		return Bass.Run(ctx, thunk, thunk.RunState(w))
	}
}

func (thunk Thunk) Export(ctx context.Context, w io.Writer) error {
	platform := thunk.Platform()

	if platform != nil {
		runtime, err := RuntimeFromContext(ctx, *platform)
		if err != nil {
			return err
		}

		return runtime.Export(ctx, w, thunk)
	} else {
		return fmt.Errorf("cannot export Bass thunk")
	}
}

func (thunk Thunk) Publish(ctx context.Context, ref ImageRef) (ImageRef, error) {
	platform := thunk.Platform()

	if platform != nil {
		runtime, err := RuntimeFromContext(ctx, *platform)
		if err != nil {
			return ref, err
		}

		return runtime.Publish(ctx, ref, thunk)
	} else {
		return ref, fmt.Errorf("cannot publish Bass thunk")
	}
}

func (thunk Thunk) Proto() (*proto.Thunk, error) {
	tp, err := thunk.MarshalProto()
	if err != nil {
		return nil, err
	}

	return tp.(*proto.Thunk), nil
}

// Start forks a goroutine that runs the thunk and calls handler with a boolean
// indicating whether it succeeded. It returns a combiner which waits for the
// thunk to finish and returns the result of the handler.
func (thunk Thunk) Start(ctx context.Context, handler Combiner) (Combiner, error) {
	ctx, stop := context.WithCancel(ctx)

	ctx = ForkTrace(ctx) // each goroutine must have its own trace

	var waitRes Value
	var waitErr error

	runs := RunsFromContext(ctx)

	wg := new(sync.WaitGroup)
	wg.Add(1)
	runs.Go(stop, func() error {
		defer wg.Done()

		runErr := thunk.Run(ctx)

		var errv Value
		if runErr != nil {
			errv = Error{runErr}
		} else {
			errv = Null{}
		}

		cont := handler.Call(ctx, NewList(errv), NewEmptyScope(), Identity)

		waitRes, waitErr = Trampoline(ctx, cont)

		return waitErr
	})

	return Func(thunk.String(), "[]", func() (Value, error) {
		wg.Wait()
		return waitRes, waitErr
	}), nil
}

func (thunk Thunk) Addr(portName Symbol, format ...string) (ThunkAddr, error) {
	var addr ThunkAddr

	var exists bool
	for _, port := range thunk.Ports {
		if port.Name == portName.String() {
			exists = true
			break
		}
	}
	if !exists {
		// TODO: better error
		return addr, fmt.Errorf("thunk %s does not have a %s port", thunk, portName)
	}

	addr.Thunk = thunk
	addr.Port = portName.String()

	if len(format) > 0 {
		addr.Format = format[0]
	} else {
		addr.Format = "$host:$port"
	}

	return addr, nil
}

func (thunk Thunk) Open(ctx context.Context) (io.ReadCloser, error) {
	// each goroutine must have its own stack
	subCtx := ForkTrace(ctx)

	r, w := io.Pipe()
	go func() {
		w.CloseWithError(thunk.Read(subCtx, w))
	}()

	return r, nil
}

// Cmdline returns a human-readable representation of the thunk's command and
// args.
func (thunk Thunk) Cmdline() string {
	var cmdline []string

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

// WithEntrypoint sets the thunk's entrypoint.
func (thunk Thunk) WithEntrypoint(entrypoint []string) Thunk {
	thunk.Entrypoint = entrypoint
	return thunk
}

// WithDefaultArgs sets the thunk's default arguments.
func (thunk Thunk) WithDefaultArgs(args []string) Thunk {
	thunk.DefaultArgs = args
	return thunk
}

// WithArgs sets the thunk's arg values.
func (thunk Thunk) WithArgs(args []Value) Thunk {
	thunk.Args = args
	return thunk
}

// AppendArgs appends to the thunk's arg values.
func (thunk Thunk) AppendArgs(args ...Value) Thunk {
	thunk.Args = append(thunk.Args, args...)
	return thunk
}

// WithEnv sets the thunk's env, replacing any existing values.
func (thunk Thunk) WithEnv(env *Scope) Thunk {
	if thunk.Env == nil {
		thunk.Env = env
	} else {
		thunk.Env = NewEmptyScope(env, thunk.Env)
	}

	return thunk
}

// WithStdin sets the thunk's stdin values.
func (thunk Thunk) WithStdin(stdin []Value) Thunk {
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

// WithLabel adds a label.
func (thunk Thunk) WithLabel(key Symbol, val Value) Thunk {
	if thunk.Labels == nil {
		thunk.Labels = NewEmptyScope()
	}

	thunk.Labels = thunk.Labels.Copy()
	thunk.Labels.Set(key, val)
	return thunk
}

// WithPorts sets the thunk's ports.
func (thunk Thunk) WithPort(name Symbol, port int) Thunk {
	thunk.Ports = append(thunk.Ports, ThunkPort{
		Name: name.String(),
		Port: port,
	})
	return thunk
}

// WithTLS configures the thunk with TLS.
func (thunk Thunk) WithTLS(cert, key FilePath) Thunk {
	thunk.TLS = &ThunkTLS{
		Cert: cert,
		Key:  key,
	}
	return thunk
}

var _ Value = Thunk{}

func (thunk Thunk) String() string {
	return fmt.Sprintf("<thunk %s: %s>", thunk.Name(), NewList(thunk.Args...))
}

func (thunk Thunk) Equal(other Value) bool {
	otherThunk, ok := other.(Thunk)
	if !ok {
		return false
	}

	msg1, err := thunk.MarshalProto()
	if err != nil {
		// not much else we can do; this should be caught in dev/test
		log.Printf("failed to marshal lhs thunk: %s", err)
		return false
	}

	msg2, err := otherThunk.MarshalProto()
	if err != nil {
		// not much else we can do; this should be caught in dev/test
		log.Printf("failed to marshal rhs thunk: %s", err)
		return false
	}

	return gproto.Equal(msg1, msg2)
}

var _ Path = Thunk{}

// Name returns the unqualified name for the path, i.e. the base name of a
// file or directory, or the name of a command.
func (thunk Thunk) Name() string {
	hash, err := thunk.Hash()
	if err != nil {
		// this is awkward, but it's better than panicking
		return fmt.Sprintf("(error: %s)", err)
	}

	return hash
}

// Extend returns a path referring to the given path relative to the parent
// Path.
func (thunk Thunk) Extend(sub Path) (Path, error) {
	return ThunkPath{
		Thunk: thunk,
		Path:  FileOrDirPath{Dir: &DirPath{"."}},
	}.Extend(sub)
}

func (thunk Thunk) Decode(dest any) error {
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

func (thunk Thunk) MarshalJSON() ([]byte, error) {
	msg, err := thunk.MarshalProto()
	if err != nil {
		return nil, err
	}

	return protojson.Marshal(msg)
}

func (thunk *Thunk) UnmarshalJSON(b []byte) error {
	msg := &proto.Thunk{}
	err := protojson.Unmarshal(b, msg)
	if err != nil {
		return err
	}

	return thunk.UnmarshalProto(msg)
}

func (thunk *Thunk) Platform() *Platform {
	if thunk.Image == nil {
		return nil
	}

	return thunk.Image.Platform()
}

// Hash returns a stable, non-cryptographic hash derived from the thunk.
func (thunk Thunk) Hash() (string, error) {
	hash, err := thunk.HashKey()
	if err != nil {
		return "", err
	}

	return b32(hash), nil
}

// Avatar returns an ASCII art avatar derived from the thunk.
func (wl Thunk) Avatar() (*invaders.Invader, error) {
	hash, err := wl.HashKey()
	if err != nil {
		return nil, err
	}

	invader := &invaders.Invader{}
	invader.Set(rand.New(rand.NewSource(int64(hash))))
	return invader, nil
}

var _ Readable = Thunk{}

func (thunk Thunk) CachePath(ctx context.Context, dest string) (string, error) {
	hash, err := thunk.Hash()
	if err != nil {
		return "", err
	}

	return Cache(ctx, filepath.Join(dest, "thunk-outputs", hash), thunk)
}

func (thunk Thunk) HashKey() (uint64, error) {
	msg, err := thunk.MarshalProto()
	if err != nil {
		return 0, err
	}

	payload, err := gproto.Marshal(msg)
	if err != nil {
		return 0, err
	}

	return xxh3.Hash(payload), nil
}

func b32(n uint64) string {
	var sum [8]byte
	binary.BigEndian.PutUint64(sum[:], n)
	return base32.HexEncoding.
		WithPadding(base32.NoPadding).
		EncodeToString(sum[:])
}
