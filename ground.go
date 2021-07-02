package bass

import (
	"context"
	"embed"
	"errors"
)

//go:embed std/*.bass
var std embed.FS

var ground = NewEnv()

func init() {
	for _, pred := range primPreds {
		ground.Set(pred.name, Func(string(pred.name), pred.check), pred.docs...)
	}

	ground.Set("ground", ground, `ground environment please ignore`,
		`This value is only here to aid in developing prior to first release.`,
		`Fetching this binding voids your warranty.`)

	ground.Set("cons",
		Func("cons", func(a, d Value) Value {
			return Pair{a, d}
		}),
		`construct a pair from the given values`)

	ground.Set("wrap",
		Func("wrap", func(c Combiner) Applicative {
			return Applicative{c}
		}),
		`construct an applicative from a combiner (typically an operative)`)

	ground.Set("unwrap",
		Func("unwrap", func(a Applicative) Combiner {
			return a.Underlying
		}),
		`access an applicative's underlying combiner`)

	ground.Set("op",
		Op("op", func(cont Cont, env *Env, formals, eformal, body Value) *Operative {
			return &Operative{
				Env:     env,
				Formals: formals,
				Eformal: eformal,
				Body:    body,
			}
		}),
		`a primitive operative constructor`,
		`op is redefined later, so no one should see this comment.`)

	ground.Set("eval",
		Applicative{Op("eval", func(cont Cont, _ *Env, val Value, env *Env) ReadyCont {
			return val.Eval(env, cont)
		})},
		`evaluate a value in an env`)

	ground.Set("make-env",
		Func("make-env", func(envs ...*Env) *Env {
			return NewEnv(envs...)
		}),
		`construct an env with the given parents`)

	ground.Set("def",
		Op("def", func(cont Cont, env *Env, formals, val Value) ReadyCont {
			return val.Eval(env, Continue(func(res Value) Value {
				err := env.Define(formals, res)
				if err != nil {
					return cont.Call(nil, err)
				}

				return cont.Call(formals, nil)
			}))
		}),
		`bind symbols to values in the current env`)

	ground.Set("doc",
		Op("doc", PrintDocs),
		`print docs for symbols`,
		`Prints the documentation for the given symbols resolved from the current environment.`,
		`With no arguments, prints the commentary for the current environment.`)

	ground.Set("if",
		Op("if", func(cont Cont, env *Env, cond, yes, no Value) ReadyCont {
			return cond.Eval(env, Continue(func(cond Value) Value {
				var res bool
				err := cond.Decode(&res)
				if err != nil {
					return yes.Eval(env, cont)
				}

				if !res {
					return no.Eval(env, cont)
				}

				return yes.Eval(env, cont)
			}))
		}),
		`if then else (branching logic)`,
		`Evaluates a condition. If nil or false, evaluates the third operand. Otherwise, evaluates the second operand.`)

	ground.Set("+",
		Func("+", func(nums ...int) int {
			sum := 0
			for _, num := range nums {
				sum += num
			}

			return sum
		}),
		`sum the given numbers`)

	ground.Set("*",
		Func("*", func(nums ...int) int {
			mul := 1
			for _, num := range nums {
				mul *= num
			}

			return mul
		}),
		`multiply the given numbers`)

	ground.Set("-",
		Func("-", func(num int, nums ...int) int {
			if len(nums) == 0 {
				return -num
			}

			sub := num
			for _, num := range nums {
				sub -= num
			}

			return sub
		}),
		`subtract ys from x`,
		`If only x is given, returns the negation of x.`)

	ground.Set("max",
		Func("max", func(num int, nums ...int) int {
			max := num
			for _, num := range nums {
				if num > max {
					max = num
				}
			}

			return max
		}),
		`largest number given`)

	ground.Set("min",
		Func("min", func(num int, nums ...int) int {
			min := num
			for _, num := range nums {
				if num < min {
					min = num
				}
			}

			return min
		}),
		`smallest number given`)

	ground.Set("=",
		Func("=", func(val Value, others ...Value) bool {
			for _, other := range others {
				if !other.Equal(val) {
					return false
				}
			}

			return true
		}),
		`numeric equality`,
	)

	ground.Set(">",
		Func(">", func(num int, nums ...int) bool {
			min := num
			for _, num := range nums {
				if num >= min {
					return false
				}

				min = num
			}

			return true
		}),
		`descending order`)

	ground.Set(">=",
		Func(">=", func(num int, nums ...int) bool {
			max := num
			for _, num := range nums {
				if num > max {
					return false
				}

				max = num
			}

			return true
		}),
		`descending or equal order`)

	ground.Set("<",
		Func("<", func(num int, nums ...int) bool {
			max := num
			for _, num := range nums {
				if num <= max {
					return false
				}

				max = num
			}

			return true
		}),
		`increasing order`)

	ground.Set("<=",
		Func("<=", func(num int, nums ...int) bool {
			max := num
			for _, num := range nums {
				if num < max {
					return false
				}

				max = num
			}

			return true
		}),
		`increasing or equal order`)

	ground.Set("*stdin*", Stdin, "A source? of values read from stdin.")
	ground.Set("*stdout*", Stdout, "A sink? for writing values to stdout.")

	ground.Set("emit",
		Func("emit", func(val Value, sink PipeSink) error {
			return sink.Emit(val)
		}),
		`send a value to a sink`,
	)

	ground.Set("next",
		Func("next", func(source PipeSource, def ...Value) (Value, error) {
			val, err := source.Next(context.Background())
			if err != nil {
				if errors.Is(err, ErrEndOfSource) && len(def) > 0 {
					return def[0], nil
				}

				return nil, err
			}

			return val, nil
		}),
		`receive the next value from a source`,
		`If the stream has ended, no value will be available. A default value may be provided, otherwise an error is raised.`,
	)

	for _, lib := range []string{
		"std/root.bass",
		"std/streams.bass",
	} {
		file, err := std.Open(lib)
		if err != nil {
			panic(err)
		}

		_, err = EvalReader(ground, file)
		if err != nil {
			panic(err)
		}
	}
}

type primPred struct {
	name  Symbol
	check func(Value) bool
	docs  []string
}

// basic predicates built in to the language.
//
// these are also used in (doc) to show which predicates a value satisfies.
var primPreds = []primPred{
	// primitive type checks
	{"null?", func(val Value) bool {
		var x Null
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is null`}},

	{"ignore?", func(val Value) bool {
		var x Ignore
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is _ ("ignore")`,
		`_ is a special value used to ignore a value when binding symbols.`,
		`For example, (def (fst . _) [1 2]) will bind 1 to fst, ignoring the rest of the list.`,
	}},

	{"boolean?", func(val Value) bool {
		var x Bool
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is true or false`}},

	{"number?", func(val Value) bool {
		var x Int
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a number`}},

	{"string?", func(val Value) bool {
		var x String
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a string`}},

	{"symbol?", func(val Value) bool {
		var x Symbol
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a symbol`}},

	{"env?", func(val Value) bool {
		var x *Env
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is an env`}},

	{"sink?", func(val Value) bool {
		var x *Sink
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a sink`,
		`A sink is a type that you can send values to using (emit).`,
	}},

	{"source?", func(val Value) bool {
		var x *Source
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a source`,
		`A source is a type that you can read values from using (next).`,
	}},

	{"list?", func(val Value) bool {
		return IsList(val)
	}, []string{
		`returns true if the value is a linked list`,
		`A linked list is a pair whose second value is another list or empty.`,
	}},

	{"pair?", func(val Value) bool {
		var x Pair
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a pair`}},

	{"object?", func(val Value) bool {
		var x Object
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is an object`,
		`An object is a mapping from keywords to values.`,
	}},

	{"keyword?", func(val Value) bool {
		var x Keyword
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is a keyword`,
		`A keyword is a constant value representing a single word with hyphens (-) translated to underscores (_).`,
	}},

	{"applicative?", func(val Value) bool {
		var x Applicative
		return val.Decode(&x) == nil
	}, []string{`returns true if the value is an applicative`,
		`An applicative is a combiner that wraps another combiner.`,
		`When an applicative is called, it evaluates its operands in the caller's evironment and passes them to the underlying combiner.`,
	}},

	{"operative?", func(val Value) bool {
		var b *Builtin
		if val.Decode(&b) == nil {
			return b.Operative
		}

		var o *Operative
		return val.Decode(&o) == nil
	}, []string{`returns true if the value is an operative`,
		`An operative is a combiner that is given the caller's environment.`,
		`An operative may decide whether and how to evaluate its arguments. They are typically used to define new syntactic constructs.`,
	}},

	{"combiner?", func(val Value) bool {
		var x Combiner
		return val.Decode(&x) == nil
	}, []string{
		`returns true if the value is a combiner`,
		`A combiner takes sequence of values as arguments and returns another value.`,
	}},

	{"empty?", func(val Value) bool {
		var empty Empty
		if err := val.Decode(&empty); err == nil {
			return true
		}

		var str string
		if err := val.Decode(&str); err == nil {
			return str == ""
		}

		var obj Object
		if err := val.Decode(&obj); err == nil {
			return len(obj) == 0
		}

		var nul Null
		if err := val.Decode(&nul); err == nil {
			return true
		}

		return false
	}, []string{
		`returns true if the value is an empty list, a zero-length string, an empty object, or null`,
	}},
}
