package bass

import "embed"

//go:embed std/*.bass
var std embed.FS

var ground = NewEnv()

func init() {
	for k, v := range primPreds {
		ground.Set(k, Func(string(k), v))
	}

	ground.Set("cons", Func("cons", func(a, d Value) Value {
		return Pair{a, d}
	}))

	ground.Set("wrap", Func("wrap", func(c Combiner) Applicative {
		return Applicative{c}
	}))

	ground.Set("unwrap", Func("unwrap", func(a Applicative) Combiner {
		return a.Underlying
	}))

	ground.Set("op", Op("op", func(env *Env, formals, eformal, body Value) *Operative {
		return &Operative{
			Env:     env,
			Formals: formals,
			Eformal: eformal,
			Body:    body,
		}
	}))

	ground.Set("eval", Func("eval", func(val Value, env *Env) (Value, error) {
		return val.Eval(env)
	}))

	ground.Set("make-env", Func("make-env", func(envs ...*Env) *Env {
		return NewEnv(envs...)
	}))

	ground.Set("def", Op("def", func(env *Env, formals, val Value) (Value, error) {
		res, err := val.Eval(env)
		if err != nil {
			return nil, err
		}

		err = env.Define(formals, res)
		if err != nil {
			return nil, err
		}

		return formals, nil
	}))

	ground.Set("if", Op("if", func(env *Env, cond, yes, no Value) (Value, error) {
		cond, err := cond.Eval(env)
		if err != nil {
			return nil, err
		}

		switch x := cond.(type) {
		case Bool:
			if !x {
				return no.Eval(env)
			}
		case Null:
			return no.Eval(env)
		}

		return yes.Eval(env)
	}))

	ground.Set("+", Func("+", func(nums ...int) int {
		sum := 0
		for _, num := range nums {
			sum += num
		}

		return sum
	}))

	ground.Set("*", Func("*", func(nums ...int) int {
		mul := 1
		for _, num := range nums {
			mul *= num
		}

		return mul
	}))

	ground.Set("-", Func("-", func(num int, nums ...int) int {
		if len(nums) == 0 {
			return -num
		}

		sub := num
		for _, num := range nums {
			sub -= num
		}

		return sub
	}))

	ground.Set("max", Func("max", func(num int, nums ...int) int {
		max := num
		for _, num := range nums {
			if num > max {
				max = num
			}
		}

		return max
	}))

	ground.Set("min", Func("min", func(num int, nums ...int) int {
		min := num
		for _, num := range nums {
			if num < min {
				min = num
			}
		}

		return min
	}))

	ground.Set("=?", Func("=?", func(cur int, nums ...int) bool {
		for _, num := range nums {
			if num != cur {
				return false
			}
		}

		return true
	}))

	ground.Set(">?", Func(">?", func(num int, nums ...int) bool {
		min := num
		for _, num := range nums {
			if num >= min {
				return false
			}

			min = num
		}

		return true
	}))

	ground.Set(">=?", Func(">=?", func(num int, nums ...int) bool {
		max := num
		for _, num := range nums {
			if num > max {
				return false
			}

			max = num
		}

		return true
	}))

	ground.Set("<?", Func("<?", func(num int, nums ...int) bool {
		max := num
		for _, num := range nums {
			if num <= max {
				return false
			}

			max = num
		}

		return true
	}))

	ground.Set("<=?", Func("<=?", func(num int, nums ...int) bool {
		max := num
		for _, num := range nums {
			if num < max {
				return false
			}

			max = num
		}

		return true
	}))

	for _, lib := range []string{
		"std/root.bass",
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

type pred func(Value) bool

var primPreds = map[Symbol]pred{
	// primitive type checks
	"null?": func(val Value) bool {
		_, is := val.(Null)
		return is
	},
	"boolean?": func(val Value) bool {
		_, is := val.(Bool)
		return is
	},
	"number?": func(val Value) bool {
		_, is := val.(Int)
		return is
	},
	"string?": func(val Value) bool {
		_, is := val.(String)
		return is
	},
	"symbol?": func(val Value) bool {
		_, is := val.(Symbol)
		return is
	},
	"env?": func(val Value) bool {
		_, is := val.(*Env)
		return is
	},
	"list?": func(val Value) bool {
		return IsList(val)
	},
	"pair?": func(val Value) bool {
		_, is := val.(Pair)
		return is
	},
	"combiner?": func(val Value) bool {
		_, is := val.(Combiner)
		return is
	},
	"applicative?": func(val Value) bool {
		_, is := val.(Applicative)
		return is
	},
	"operative?": func(val Value) bool {
		switch x := val.(type) {
		case *Builtin:
			return x.Operative
		case *Operative:
			return true
		default:
			return false
		}
	},
	"empty?": func(val Value) bool {
		switch x := val.(type) {
		case Empty:
			return true
		case String:
			return len(x) == 0
		case Null:
			return true
		default:
			return false
		}
	},
}
