package bass

func New() *Env {
	env := NewEnv()

	env.Set("+", Func("+", func(nums ...int) int {
		sum := 0
		for _, num := range nums {
			sum += num
		}
		return sum
	}))

	return env
}
