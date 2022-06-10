package cli_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/basstest"
	"github.com/vito/bass/pkg/cli"
	"github.com/vito/is"
)

func TestInputsSource(t *testing.T) {
	for _, example := range []struct {
		Inputs []string
		Value  bass.Value
	}{
		{[]string{"bool"}, bass.Bindings{
			"bool": bass.Bool(true),
		}.Scope()},
		{[]string{"str=abc"}, bass.Bindings{
			"str": bass.String("abc"),
		}.Scope()},
		{[]string{"str=abc"}, bass.Bindings{
			"str": bass.String("abc"),
		}.Scope()},
		{[]string{"bool", "str=abc"}, bass.Bindings{
			"bool": bass.Bool(true),
			"str":  bass.String("abc")}.Scope(),
		},
		{[]string{"path=./"}, bass.Bindings{
			"path": bass.NewHostPath(".", bass.ParseFileOrDirPath(".")),
		}.Scope()},
		{[]string{"notpath=."}, bass.Bindings{
			"notpath": bass.String("."),
		}.Scope()},
		{[]string{"path=/absolute"}, bass.Bindings{
			"path": bass.NewHostPath("/", bass.ParseFileOrDirPath("absolute")),
		}.Scope()},
		{[]string{"path=/absolute/file"}, bass.Bindings{
			"path": bass.NewHostPath("/absolute", bass.ParseFileOrDirPath("file")),
		}.Scope()},
		{[]string{"path=/absolute/dir/"}, bass.Bindings{
			"path": bass.NewHostPath("/absolute/dir", bass.ParseFileOrDirPath(".")),
		}.Scope()},
		{[]string{"path=/absolute/dir/"}, bass.Bindings{
			"path": bass.NewHostPath("/absolute/dir", bass.ParseFileOrDirPath(".")),
		}.Scope()},
		{[]string{"path=./relative/dir/"}, bass.Bindings{
			"path": bass.NewHostPath("relative/dir", bass.ParseFileOrDirPath(".")),
		}.Scope()},
		{[]string{"path=./relative/file"}, bass.Bindings{
			"path": bass.NewHostPath("relative", bass.ParseFileOrDirPath("file")),
		}.Scope()},
		{[]string{"notpath=ambiguous/path/"}, bass.Bindings{
			"notpath": bass.String("ambiguous/path/"),
		}.Scope()},
		{[]string{"notpath=ambiguous/path"}, bass.Bindings{
			"notpath": bass.String("ambiguous/path"),
		}.Scope()},
		{[]string{"uri=http://example.com/"}, bass.Bindings{
			"uri": bass.String("http://example.com/"),
		}.Scope()},
	} {
		t.Run(fmt.Sprintf("%q", example.Inputs), func(t *testing.T) {
			is := is.New(t)

			source := cli.InputsSource(example.Inputs)

			val, err := source.PipeSource.Next(context.TODO())
			is.NoErr(err)

			basstest.Equal(t, example.Value, val)
		})
	}
}
