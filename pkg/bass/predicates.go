package bass

// IsOperative returns true iff the value is an Operative or a Builtin
// operative.
func IsOperative(val Value) bool {
	var b *Builtin
	if val.Decode(&b) == nil {
		return b.Operative
	}

	var o *Operative
	return val.Decode(&o) == nil
}

// IsApplicative returns true iff the value is an Applicative.
func IsApplicative(val Value) bool {
	var x Applicative
	return val.Decode(&x) == nil
}
