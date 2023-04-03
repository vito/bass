package bass_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

var encodable = []bass.Value{
	bass.Null{},
	bass.Empty{},
	bass.Bool(true),
	bass.Bool(false),
	bass.Int(42),
	bass.NewList(
		bass.Bool(true),
		bass.Int(1),
		bass.String("hello"),
	),
	bass.NewEmptyScope(),
	bass.Bindings{
		"a": bass.Bool(true),
		"b": bass.Int(1),
		"c": bass.String("hello"),
	}.Scope(),
	bass.Bindings{
		"hyphenated-key": bass.String("hello"),
	}.Scope(),
	bass.NewList(
		bass.Bool(true),
		bass.Int(1),
		bass.Bindings{
			"a": bass.Bool(true),
			"b": bass.Int(1),
			"c": bass.String("hello"),
		}.Scope(),
	),
	bass.Bindings{
		"a": bass.Bool(true),
		"b": bass.Int(1),
		"c": bass.NewList(
			bass.Bool(true),
			bass.Int(1),
			bass.String("hello"),
		),
	}.Scope(),
	bass.DirPath{"directory-path"},
	bass.FilePath{"file-path"},
	bass.CommandPath{"command-path"},
	bass.NewHostPath("./", bass.ParseFileOrDirPath("foo")),
	validBasicThunk,
	bass.ThunkPath{
		Thunk: validBasicThunk,
		Path:  bass.ParseFileOrDirPath("thunk/file"),
	},
	bass.ThunkPath{
		Thunk: validBasicThunk,
		Path:  bass.ParseFileOrDirPath("thunk/dir/"),
	},
	validThiccThunk,
}

// minimum viable thunk
var validScratchThunk = bass.Thunk{}

var validBasicThunk = bass.Thunk{
	Args: []bass.Value{bass.FilePath{"basic"}},
}

// avoid using bass.Bindings{} so the order is stable
var stableEnv = bass.NewEmptyScope()

func init() {
	stableEnv.Set("B-ENV", bass.String("sup"))
	stableEnv.Set("A-DIR", bass.ThunkPath{
		Thunk: validBasicThunk,
		Path:  bass.ParseFileOrDirPath("env/path/"),
	})
}

// avoid using bass.Bindings{} so the order is stable
var stableLabels = bass.NewEmptyScope()

func init() {
	stableLabels.Set("b-some", bass.String("label"))
	stableLabels.Set("a-at", bass.String("now"))
}

// a thunk with all "simple" (non-enum) fields filled-in
var validThiccThunk = bass.Thunk{
	Args: []bass.Value{
		bass.FilePath{"run"},
		bass.String("arg"),
		bass.ThunkPath{
			Thunk: bass.Thunk{Args: []bass.Value{

				bass.FilePath{"basic"}}},

			Path: bass.ParseFileOrDirPath("arg/path/"),
		},
	},
	Stdin: []bass.Value{
		bass.String("stdin"),
		bass.ThunkPath{
			Thunk: bass.Thunk{Args: []bass.Value{

				bass.FilePath{"basic"}}},

			Path: bass.ParseFileOrDirPath("stdin/path/"),
		},
	},
	Env:    stableEnv,
	Labels: stableLabels,
	Ports: []bass.ThunkPort{
		{"http", 80},
		{"ssh", 22},
	},
	TLS: &bass.ThunkTLS{
		Cert: bass.FilePath{"cert"},
		Key:  bass.FilePath{"key"},
	},
}

var validThunkImages = []bass.ThunkImage{
	{
		Thunk: &validScratchThunk,
	},
	{
		Thunk: &validBasicThunk,
	},
}

var validThunkImageRefs = []bass.ImageRef{
	{
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		Repository: bass.ImageRepository{
			Static: "repo",
		},
		Tag:    "tag",
		Digest: "digest",
	},
	{
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		Repository: bass.ImageRepository{
			Static: "repo",
		},
		Tag: "tag",
		// no digest
	},
	{
		Platform: bass.Platform{
			OS: "os",
			// no arch
		},
		Repository: bass.ImageRepository{
			Static: "repo",
		},
		Tag: "tag",
		// no digest
	},
	{
		Platform: bass.Platform{
			OS: "os",
			// no arch
		},
		Repository: bass.ImageRepository{
			Static: "repo",
		},
		// no tag
		// no digest
	},
	{
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		Repository: bass.ImageRepository{
			Addr: &bass.ThunkAddr{
				Thunk:  validBasicThunk,
				Port:   "http",
				Format: "$host:$port/repo",
			},
		},
		Tag:    "tag",
		Digest: "digest",
	},
	{
		Platform: bass.Platform{
			OS: "os",
			// no arch
		},
		Repository: bass.ImageRepository{
			Addr: &bass.ThunkAddr{
				Thunk:  validBasicThunk,
				Port:   "http",
				Format: "$host:$port/repo",
			},
		},
		// no tag
		// no digest
	},
}

var validThunkImageArchives = []bass.ImageArchive{
	{
		File: bass.ThunkPath{
			Thunk: validBasicThunk,
			Path:  bass.ParseFileOrDirPath("image.tar"),
		},
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		Tag: "tag",
	},
	{
		File: bass.ThunkPath{
			Thunk: validBasicThunk,
			Path:  bass.ParseFileOrDirPath("image.tar"),
		},
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		// no tag
	},
}

var validThunkImageDockerBuilds = []bass.ImageDockerBuild{
	{
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		Context: bass.ImageBuildInput{
			Thunk: &bass.ThunkPath{
				Thunk: validBasicThunk,
				Path:  bass.ParseFileOrDirPath("thunk/dir/"),
			},
		},
		Dockerfile: bass.NewFilePath("my-dockerfile"),
		Target:     "target",
		Args: bass.Bindings{
			"arg1": bass.String("value1"),
			"arg2": bass.String("value2"),
		}.Scope(),
	},
	{
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
		Context: bass.ImageBuildInput{
			Host: &bass.HostPath{
				ContextDir: "context-dir",
				Path:       bass.ParseFileOrDirPath("host/dir/"),
			},
		},
	},
	{
		Context: bass.ImageBuildInput{
			FS: bass.NewInMemoryFile("fs/mount-dir/file", "hello").Dir(),
		},
		Platform: bass.Platform{
			OS:           "os",
			Architecture: "arch",
		},
	},
}

func init() {
	for _, ref := range validThunkImageRefs {
		cp := ref
		validThunkImages = append(validThunkImages, bass.ThunkImage{
			Ref: &cp,
		})
	}

	for _, ref := range validThunkImageArchives {
		cp := ref
		validThunkImages = append(validThunkImages, bass.ThunkImage{
			Archive: &cp,
		})
	}

	for _, ref := range validThunkImageDockerBuilds {
		cp := ref
		validThunkImages = append(validThunkImages, bass.ThunkImage{
			DockerBuild: &cp,
		})
	}

	for _, img := range validThunkImages {
		thunk := validBasicThunk
		cp := img
		thunk.Image = &cp
		encodable = append(encodable, thunk)
	}
}

var validThunkCmds = []bass.Value{
	bass.CommandPath{"cmd"},
	bass.FilePath{"file"},
	bass.ThunkPath{
		Thunk: validBasicThunk,
		Path:  bass.ParseFileOrDirPath("thunk/file"),
	},
	bass.HostPath{
		ContextDir: "context-dir",
		Path:       bass.ParseFileOrDirPath("host/file"),
	},
	bass.NewInMemoryFile("fs/dir/cmd-file", "hello"),
}

func init() {
	// no command
	encodable = append(encodable, validScratchThunk)

	// all commands
	for _, cmd := range validThunkCmds {
		thunk := validBasicThunk
		thunk.Args = append([]bass.Value{cmd}, thunk.Args...)
		encodable = append(encodable, thunk)
	}
}

var validThunkDirs = []bass.ThunkDir{
	{
		ThunkDir: &bass.ThunkPath{
			Thunk: validBasicThunk,
			Path:  bass.ParseFileOrDirPath("dir/"),
		},
	},
	{
		HostDir: &bass.HostPath{
			ContextDir: "context-dir",
			Path:       bass.ParseFileOrDirPath("dir/"),
		},
	},
	{
		ThunkDir: &bass.ThunkPath{
			Thunk: validBasicThunk,
			Path:  bass.ParseFileOrDirPath("dir/"),
		},
	},
}

func init() {
	for _, dir := range validThunkDirs {
		thunk := validBasicThunk
		cp := dir
		thunk.Dir = &cp
		encodable = append(encodable, thunk)
	}
}

var validThunkMountSources = []bass.ThunkMountSource{
	{
		ThunkPath: &bass.ThunkPath{
			Thunk: validBasicThunk,
			Path:  bass.ParseFileOrDirPath("thunk/dir/"),
		},
	},
	{
		HostPath: &bass.HostPath{
			ContextDir: "context",
			Path:       bass.ParseFileOrDirPath("host/dir/"),
		},
	},
	{
		FSPath: bass.NewInMemoryFile("fs/mount-dir/file", "hello").Dir(),
	},
	{
		Cache: &bass.CachePath{
			ID: "some-cache",
			Path: bass.FileOrDirPath{
				Dir: &bass.DirPath{"cache/dir"},
			},
		},
	},
	{
		Secret: &bass.Secret{
			Name: "some-secret",
		},
	},
}

func init() {
	for _, src := range validThunkMountSources {
		thunk := validBasicThunk
		thunk.Mounts = append(
			thunk.Mounts,
			bass.ThunkMount{
				Source: src,
				Target: bass.ParseFileOrDirPath("mount/dir/"),
			},
			bass.ThunkMount{
				Source: src,
				Target: bass.ParseFileOrDirPath("mount/file"),
			},
		)
		encodable = append(encodable, thunk)
	}
}

func TestProtoable(t *testing.T) {
	for _, val := range encodable {
		val := val

		ptr := reflect.New(reflect.TypeOf(val))

		marshaler, ok := val.(bass.ProtoMarshaler)
		if !ok {
			continue
		}

		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			is := is.New(t)

			msg, err := marshaler.MarshalProto()
			is.NoErr(err)

			unmarshaler, ok := ptr.Interface().(bass.ProtoUnmarshaler)
			if ok {
				err := unmarshaler.UnmarshalProto(msg)
				is.NoErr(err)
				basstest.Equal(t, ptr.Elem().Interface().(bass.Value), val)
			}
		})
	}
}

func TestJSONable(t *testing.T) {
	for _, val := range encodable {
		val := val
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			testJSONValueDecodeLifecycle(t, val)
		})
	}
}

func TestUnJSONable(t *testing.T) {
	for _, val := range []bass.Value{
		bass.Op("noop", "[]", func() {}),
		bass.Func("nofn", "[]", func() {}),
		operative,
		bass.Wrapped{operative},
		bass.Stdin,
		bass.Stdout,
		&bass.Continuation{
			Continue: func(x bass.Value) bass.Value {
				return x
			},
		},
		&bass.ReadyContinuation{
			Cont: &bass.Continuation{
				Continue: func(x bass.Value) bass.Value {
					return x
				},
			},
			Result: bass.Int(42),
		},
		bass.Pair{
			A: bass.String("a"),
			D: bass.String("d"),
		},
		bass.Cons{
			A: bass.String("a"),
			D: bass.String("d"),
		},
		bass.Bind{
			bass.Pair{
				A: bass.String("a"),
				D: bass.String("d"),
			},
		},
		bass.Annotate{
			Value:   bass.String("foo"),
			Comment: "annotated",
		},
	} {
		val := val
		t.Run(fmt.Sprintf("%T", val), func(t *testing.T) {
			is := is.New(t)

			_, err := bass.MarshalJSON(val)
			is.True(err != nil)

			var marshalErr bass.EncodeError
			is.True(errors.As(err, &marshalErr))
			is.Equal(marshalErr.Value, val)
		})
	}
}

func testJSONValueDecodeLifecycle(t *testing.T, val bass.Value) {
	t.Run("basic marshaling", func(t *testing.T) {
		is := is.New(t)

		type_ := reflect.TypeOf(val)

		payload, err := bass.MarshalJSON(val)
		is.NoErr(err)

		t.Logf("typed value -> json: %s", string(payload))

		dest := reflect.New(type_)
		err = json.Unmarshal(payload, dest.Interface())
		is.NoErr(err)

		t.Logf("json -> typed value: %+v", dest.Interface())

		equalSameType(t, val, dest.Elem().Interface().(bass.Value))
	})

	t.Run("in a struct", func(t *testing.T) {
		is := is.New(t)

		structType := reflect.StructOf([]reflect.StructField{
			{
				Name: "Value",
				Type: reflect.TypeOf(val),
				Tag:  `json:"value"`,
			},
		})

		object := reflect.New(structType)
		object.Elem().Field(0).Set(reflect.ValueOf(val))

		t.Logf("value -> struct: %+v", object.Interface())

		payload, err := bass.MarshalJSON(object.Interface())
		is.NoErr(err)

		t.Logf("struct -> json: %s", string(payload))

		dest := reflect.New(structType)
		err = json.Unmarshal(payload, dest.Interface())
		is.NoErr(err)

		t.Logf("json -> struct: %+v", dest.Interface())

		equalSameType(t, val, dest.Elem().Field(0).Interface().(bass.Value))
	})
}

func equalSameType(t *testing.T, expected, actual bass.Value) {
	t.Helper()
	is.New(t).Equal(reflect.TypeOf(expected), reflect.TypeOf(actual))
	Equal(t, expected, actual)
}
