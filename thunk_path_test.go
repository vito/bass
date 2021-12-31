package bass_test

import (
	"encoding/json"
	"testing"

	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
	"github.com/vito/is"
)

func TestThunkPathJSON(t *testing.T) {
	is := is.New(t)

	wlp := bass.ThunkPath{
		Thunk: bass.Thunk{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	payload, err := json.Marshal(wlp)
	is.NoErr(err)

	var wlp2 bass.ThunkPath
	err = json.Unmarshal(payload, &wlp2)
	is.NoErr(err)

	Equal(t, wlp, wlp2)

	// an empty JSON object must fail on missing keys
	err = json.Unmarshal([]byte(`{}`), &wlp2)
	is.True(err != nil)
}

func TestThunkPathEqual(t *testing.T) {
	is := is.New(t)

	wlp := bass.ThunkPath{
		Thunk: bass.Thunk{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	diffPath := wlp
	diffPath.Path = bass.FileOrDirPath{
		File: &bass.FilePath{"foo"},
	}

	Equal(t, wlp, wlp)
	is.True(!wlp.Equal(diffPath))
}

func TestThunkPathDecode(t *testing.T) {
	is := is.New(t)

	wlp := bass.ThunkPath{
		Thunk: bass.Thunk{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	var foo bass.ThunkPath
	err := wlp.Decode(&foo)
	is.NoErr(err)
	is.Equal(foo, wlp)

	var path_ bass.Path
	err = wlp.Decode(&path_)
	is.NoErr(err)
	is.Equal(path_, wlp)

	var comb bass.Combiner
	err = wlp.Decode(&comb)
	is.NoErr(err)
	is.Equal(comb, wlp)

	var app bass.Applicative
	err = wlp.Decode(&app)
	is.NoErr(err)
	is.Equal(comb, wlp)
}

func TestThunkPathCall(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := bass.ThunkPath{
		Thunk: bass.Thunk{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"foo"},
		},
	}

	scope.Set("foo", bass.String("hello"))

	res, err := Call(val, scope, bass.NewList(bass.Symbol("foo")))
	is.NoErr(err)
	Equal(t, res, bass.Bindings{
		"path":  val,
		"stdin": bass.NewList(bass.String("hello")),
	}.Scope())

}

func TestThunkPathUnwrap(t *testing.T) {
	is := is.New(t)

	scope := bass.NewEmptyScope()
	val := bass.ThunkPath{
		Thunk: bass.Thunk{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"foo"},
		},
	}

	res, err := Call(val.Unwrap(), scope, bass.NewList(bass.String("hello")))
	is.NoErr(err)
	Equal(t, res, bass.Bindings{
		"path":  val,
		"stdin": bass.NewList(bass.String("hello")),
	}.Scope())

}

func TestThunkPathName(t *testing.T) {
	is := is.New(t)

	wl := bass.Thunk{
		Path: bass.RunPath{
			File: &bass.FilePath{"run"},
		},
	}

	is.Equal(
		"foo",
		bass.ThunkPath{
			Thunk: wl,
			Path: bass.FileOrDirPath{
				Dir: &bass.DirPath{"foo"},
			},
		}.Name(),
	)
}

func TestThunkPathExtend(t *testing.T) {
	is := is.New(t)

	var parent, child bass.Path

	wl := bass.Thunk{
		Path: bass.RunPath{
			File: &bass.FilePath{"run"},
		},
	}

	parent = bass.ThunkPath{
		Thunk: wl,
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	child = bass.DirPath{"bar"}
	sub, err := parent.Extend(child)
	is.NoErr(err)
	is.Equal(sub, bass.ThunkPath{
		Thunk: wl,
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo/bar"},
		},
	})

	child = bass.FilePath{"bar"}
	sub, err = parent.Extend(child)
	is.NoErr(err)
	is.Equal(sub, bass.ThunkPath{
		Thunk: wl,
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"foo/bar"},
		},
	})

	child = bass.CommandPath{"bar"}
	sub, err = parent.Extend(child)
	is.True(sub == nil)
	is.True(err != nil)
}
