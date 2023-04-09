package bass_test

import (
	"encoding/json"
	"testing"

	"github.com/vito/bass/pkg/bass"
	. "github.com/vito/bass/pkg/basstest"
	"github.com/vito/is"
)

func TestThunkPathJSON(t *testing.T) {
	is := is.New(t)

	wlp := bass.ThunkPath{
		Thunk: bass.Thunk{
			Args: []bass.Value{
				bass.FilePath{"run"},
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
}

func TestThunkPathEqual(t *testing.T) {
	is := is.New(t)

	wlp := bass.ThunkPath{
		Thunk: bass.Thunk{
			Args: []bass.Value{
				bass.FilePath{"run"},
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
			Args: []bass.Value{
				bass.FilePath{"run"},
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

func TestThunkPathName(t *testing.T) {
	is := is.New(t)

	wl := bass.Thunk{
		Args: []bass.Value{
			bass.FilePath{"run"},
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
		Args: []bass.Value{
			bass.FilePath{"run"},
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
