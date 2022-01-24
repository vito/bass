package hl

import (
	"fmt"
	"strings"

	"github.com/vito/bass/pkg/bass"
)

//go:generate stringer -type=Class

type Class int

const (
	Invalid Class = iota
	Bool
	Const
	Cond
	Repeat
	Var
	Def
	Fn
	Op
	Special
)

type Classes map[Class][]bass.Symbol

func Classify(scope *bass.Scope) Classes {
	cs := Classes{}

	for class, names := range staticClasses {
		cs[class] = names
	}

	for class := range dynamicClasses {
		cs[class] = Bindings(scope, class)
	}

	return cs
}

func Bindings(scope *bass.Scope, class Class) []bass.Symbol {
	if names, found := staticClasses[class]; found {
		return names
	}

	fn, found := dynamicClasses[class]
	if !found {
		panic(fmt.Errorf("unknown class: %s", class))
	}

	names := []bass.Symbol{}
	_ = scope.Each(func(s bass.Symbol, v bass.Value) error {
		if fn(s, v) {
			names = append(names, s)
		}

		return nil
	})

	return names
}

var staticClasses = Classes{
	Bool:   {"true", "false"},
	Const:  {"null", "_"},
	Cond:   {"case", "cond"},
	Repeat: {"each"},
}

type classifyFunc func(bass.Symbol, bass.Value) bool

var dynamicClasses = map[Class]classifyFunc{
	Def: func(s bass.Symbol, _ bass.Value) bool {
		return !isStatic(s) && isDefine(s)
	},
	Fn: func(s bass.Symbol, v bass.Value) bool {
		return !isStatic(s) && bass.IsApplicative(v)
	},
	Op: func(s bass.Symbol, v bass.Value) bool {
		// must not include builtin ops so that bassSpecial takes precedence
		var op *bass.Operative
		return !isStatic(s) && !isDefine(s) && v.Decode(&op) == nil
	},
	Special: func(s bass.Symbol, v bass.Value) bool {
		var builtin *bass.Builtin
		return !isStatic(s) && !isDefine(s) && v.Decode(&builtin) == nil
	},
	Var: func(s bass.Symbol, _ bass.Value) bool {
		str := string(s)
		return strings.HasPrefix(str, "*") && strings.HasSuffix(str, "*") && str != "*"
	},
}

func isDefine(s bass.Symbol) bool {
	return strings.HasPrefix(s.String(), "def")
}

func isStatic(s bass.Symbol) bool {
	for _, names := range staticClasses {
		for _, n := range names {
			if n == s {
				return true
			}
		}
	}

	return false
}
