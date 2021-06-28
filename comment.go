package bass

type Commented struct {
	Comment string

	Value
}

func (value Commented) Eval(env *Env) (Value, error) {
	res, err := value.Value.Eval(env)
	if err != nil {
		return nil, err
	}

	env.Commentary = append(env.Commentary, Commented{
		Comment: value.Comment,
		Value:   res,
	})

	return res, nil
}
