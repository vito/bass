package bass

func New() *Env {
	env := NewEnv()

	for k, v := range primPreds {
		env.Set(k, Func(string(k), v))
	}

	env.Set("+", Func("+", func(nums ...int) int {
		sum := 0
		for _, num := range nums {
			sum += num
		}

		return sum
	}))

	return env
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
		_, is := val.(List)
		return is
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
