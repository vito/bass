package bass_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vito/bass"
	. "github.com/vito/bass/basstest"
)

func TestWorkloadPathJSON(t *testing.T) {
	wlp := bass.WorkloadPath{
		Workload: bass.Workload{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	payload, err := json.Marshal(wlp)
	require.NoError(t, err)

	var wlp2 bass.WorkloadPath
	err = json.Unmarshal(payload, &wlp2)
	require.NoError(t, err)

	Equal(t, wlp, wlp2)
}

func TestWorkloadPathEqual(t *testing.T) {
	wlp := bass.WorkloadPath{
		Workload: bass.Workload{
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

	require.True(t, wlp.Equal(wlp))
	require.False(t, wlp.Equal(diffPath))
}

func TestWorkloadPathDecode(t *testing.T) {
	wlp := bass.WorkloadPath{
		Workload: bass.Workload{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	var foo bass.WorkloadPath
	err := wlp.Decode(&foo)
	require.NoError(t, err)
	require.Equal(t, wlp, foo)

	var path_ bass.Path
	err = wlp.Decode(&path_)
	require.NoError(t, err)
	require.Equal(t, wlp, path_)

	var comb bass.Combiner
	err = wlp.Decode(&comb)
	require.NoError(t, err)
	require.Equal(t, wlp, comb)

	var app bass.Applicative
	err = wlp.Decode(&app)
	require.NoError(t, err)
	require.Equal(t, wlp, comb)
}

func TestWorkloadPathCall(t *testing.T) {
	env := bass.NewEnv()
	val := bass.WorkloadPath{
		Workload: bass.Workload{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"foo"},
		},
	}

	env.Set("foo", bass.String("hello"))

	res, err := Call(val, env, bass.NewList(bass.Symbol("foo")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Object{
		"path":     val,
		"stdin":    bass.NewList(bass.String("hello")),
		"response": bass.Object{"stdout": bass.Bool(true)},
	})
}

func TestWorkloadPathUnwrap(t *testing.T) {
	env := bass.NewEnv()
	val := bass.WorkloadPath{
		Workload: bass.Workload{
			Path: bass.RunPath{
				File: &bass.FilePath{"run"},
			},
		},
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"foo"},
		},
	}

	res, err := Call(val.Unwrap(), env, bass.NewList(bass.String("hello")))
	require.NoError(t, err)
	require.Equal(t, res, bass.Object{
		"path":     val,
		"stdin":    bass.NewList(bass.String("hello")),
		"response": bass.Object{"stdout": bass.Bool(true)},
	})
}

func TestWorkloadPathExtend(t *testing.T) {
	var parent, child bass.Path

	wl := bass.Workload{
		Path: bass.RunPath{
			File: &bass.FilePath{"run"},
		},
	}

	parent = bass.WorkloadPath{
		Workload: wl,
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo"},
		},
	}

	child = bass.DirPath{"bar"}
	sub, err := parent.Extend(child)
	require.NoError(t, err)
	require.Equal(t, bass.WorkloadPath{
		Workload: wl,
		Path: bass.FileOrDirPath{
			Dir: &bass.DirPath{"foo/bar"},
		},
	}, sub)

	child = bass.FilePath{"bar"}
	sub, err = parent.Extend(child)
	require.NoError(t, err)
	require.Equal(t, bass.WorkloadPath{
		Workload: wl,
		Path: bass.FileOrDirPath{
			File: &bass.FilePath{"foo/bar"},
		},
	}, sub)

	child = bass.CommandPath{"bar"}
	sub, err = parent.Extend(child)
	require.Nil(t, sub)
	require.Error(t, err)
}
