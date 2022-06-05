package proto

func Resolve(val *Value, r func(*Value) (*Value, error)) (*Value, error) {
	val, err := r(val)
	if err != nil {
		return nil, err
	}

	switch x := val.GetValue().(type) {
	case *Value_ArrayValue:
		cp := &Array{}
		for _, v := range x.ArrayValue.GetValues() {
			val, err = Resolve(v, r)
			if err != nil {
				return nil, err
			}

			cp.Values = append(cp.Values, val)
		}

		return NewValue(cp)
	case *Value_ObjectValue:
		cp := &Object{}
		for _, v := range x.ObjectValue.GetBindings() {
			val, err = Resolve(v.Value, r)
			if err != nil {
				return nil, err
			}

			cp.Bindings = append(cp.Bindings, &Binding{
				Name:  v.Name,
				Value: val,
			})
		}

		return NewValue(cp)
	}

	return val, nil
}
