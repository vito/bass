package bass_test

import (
	"testing"

	"github.com/vito/bass/pkg/bass"
)

func TestProtocols(t *testing.T) {
	for _, e := range []BasicExample{
		{
			Name: "raw",
			Bass: `(take-all (read (mkfile ./foo "hello\nworld\n") :raw))`,
			Result: bass.NewList(
				bass.String("hello\nworld\n"),
			),
		},
		{
			Name:   "lines",
			Bass:   `(take-all (read (mkfile ./foo "hello\nworld\n") :lines))`,
			Result: bass.NewList(bass.String("hello"), bass.String("world")),
		},
		{
			Name:   "lines includes blank lines",
			Bass:   `(take-all (read (mkfile ./foo "hello\n\nworld\n") :lines))`,
			Result: bass.NewList(bass.String("hello"), bass.String(""), bass.String("world")),
		},
		{
			Name:   "lines includes last unterminated line",
			Bass:   `(take-all (read (mkfile ./foo "hello\nworld") :lines))`,
			Result: bass.NewList(bass.String("hello"), bass.String("world")),
		},
		{
			Name: "unix-table",
			Bass: `(take-all (read (mkfile ./foo "hello world\ngoodbye universe\n") :unix-table))`,
			Result: bass.NewList(
				bass.NewList(bass.String("hello"), bass.String("world")),
				bass.NewList(bass.String("goodbye"), bass.String("universe")),
			),
		},
		{
			Name: "unix-table includes empty rows",
			Bass: `(take-all (read (mkfile ./foo "hello world\n\ngoodbye universe\n") :unix-table))`,
			Result: bass.NewList(
				bass.NewList(bass.String("hello"), bass.String("world")),
				bass.NewList(),
				bass.NewList(bass.String("goodbye"), bass.String("universe")),
			),
		},
		{
			Name: "unix-table includes last unterminated row",
			Bass: `(take-all (read (mkfile ./foo "hello world\ngoodbye universe") :unix-table))`,
			Result: bass.NewList(
				bass.NewList(bass.String("hello"), bass.String("world")),
				bass.NewList(bass.String("goodbye"), bass.String("universe")),
			),
		},
		{
			Name: "json",
			Bass: `(take-all (read (mkfile ./foo "{\"a\":1}{\"b\":\"two\"}") :json))`,
			Result: bass.NewList(
				bass.Bindings{"a": bass.Int(1)}.Scope(),
				bass.Bindings{"b": bass.String("two")}.Scope(),
			),
		},
	} {
		e.Run(t)
	}
}
