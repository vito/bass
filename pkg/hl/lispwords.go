package hl

import "github.com/vito/bass/pkg/bass"

// IndentMetaBinding is set to true in meta to indicate multiline forms using
// the combiner should have Vim lispwords style indentation, i.e. a 2 space
// indent instead of aligning with first argument.
const IndentMetaBinding bass.Symbol = "indent"

// Lispwords collects bindings from the scope whose value has IndentMetaBinding
// set to true.
func LispWords(scope *bass.Scope) []bass.Symbol {
	names := []bass.Symbol{}
	_ = scope.Each(func(s bass.Symbol, v bass.Value) error {
		var ann bass.Annotated
		if v.Decode(&ann) != nil {
			return nil
		}

		var indent bool
		if ann.Meta.GetDecode(IndentMetaBinding, &indent) != nil {
			return nil
		}

		if indent {
			names = append(names, s)
		}

		return nil
	})

	return names
}
