package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/is"
)

func TestRun(t *testing.T) {
	for _, test := range []struct {
		name    string
		env     *bass.Scope
		inputs  []string
		helpers map[string]string
		script  string
		argv    []string
		stdout  []bass.Value
		err     error
	}{
		{name: "empty", script: ``},
		{
			name:   "main",
			script: `(defn main [] (emit 42 *stdout*))`,
			stdout: []bass.Value{bass.Int(42)},
		},
		{
			name:   "argv",
			script: `(defn main [val] (emit val *stdout*))`,
			argv:   []string{"hello"},
			stdout: []bass.Value{bass.String("hello")},
		},
		{
			name:   "env",
			script: `(defn main [] (emit *env*:FOO *stdout*))`,
			env:    bass.Bindings{"FOO": bass.String("hello")}.Scope(),
			stdout: []bass.Value{bass.String("hello")},
		},
		{
			name:   "inputs",
			script: `(defn main [] (for [params *stdin*] (emit params *stdout*)))`,
			inputs: []string{"str=hello", "path=./foo"},
			stdout: []bass.Value{
				bass.Bindings{
					"str":  bass.String("hello"),
					"path": bass.NewHostPath(".", bass.ParseFileOrDirPath("./foo")),
				}.Scope(),
			},
		},
		{
			name: "waiting on started thunks propagates errors",
			helpers: map[string]string{
				"fail.bass": `(defn main [] (error "boom"))`,
			},
			script: `(defn main [] (start (*dir*/fail.bass) (fn [err] (and err (err)))) (emit 42 *stdout*) (wait))`,
			stdout: []bass.Value{bass.Int(42)},
			err: &bass.StructuredError{
				Message: "boom",
				Fields:  bass.NewEmptyScope(),
			},
		},
		{
			name: "waiting after using succeeds does not error",
			helpers: map[string]string{
				"fail.bass": `(defn main [] (error "boom"))`,
			},
			script: `(defn main [] (emit (succeeds? (*dir*/fail.bass)) *stdout*) (wait))`,
			stdout: []bass.Value{bass.Bool(false)},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			is := is.New(t)

			tmp := t.TempDir()
			script := filepath.Join(tmp, test.name+".bass")
			is.NoErr(os.WriteFile(script, []byte(test.script), 0644))
			for fn, content := range test.helpers {
				script := filepath.Join(tmp, fn)
				is.NoErr(os.WriteFile(script, []byte(content), 0644))
			}

			stdout := bass.NewInMemorySink()
			runErr := cli.Run(context.Background(), test.env, test.inputs, script, test.argv, bass.NewSink(stdout))
			if test.err != nil {
				is.Equal(test.err, runErr)
			} else {
				is.NoErr(runErr)
			}

			is.Equal(len(test.stdout), len(stdout.Values))
			for i, v := range test.stdout {
				basstest.Equal(t, v, stdout.Values[i])
			}
		})
	}
}
