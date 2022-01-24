package plugin

import (
	"fmt"

	"github.com/alecthomas/chroma"
	. "github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/vito/bass/pkg/bass"
	"github.com/vito/bass/pkg/hl"
	"github.com/vito/bass/pkg/runtimes"
)

var BassLexer = lexers.Register(MustNewLazyLexer(
	&Config{
		Name:      "Bass",
		Aliases:   []string{"bass"},
		Filenames: []string{"*.bass"},
		MimeTypes: []string{"text/x-bass", "application/x-bass"},
	},
	bassRules,
))

var class2chroma = map[hl.Class]chroma.TokenType{
	hl.Bool:    chroma.KeywordConstant,
	hl.Const:   chroma.KeywordConstant,
	hl.Cond:    chroma.Keyword,
	hl.Repeat:  chroma.NameBuiltin,
	hl.Var:     chroma.NameBuiltinPseudo,
	hl.Def:     chroma.KeywordDeclaration,
	hl.Fn:      chroma.NameFunction,
	hl.Op:      chroma.NameBuiltin,
	hl.Special: chroma.Keyword,
}

const symChars = `\w!$%*+<=>?.#-`

func bassRules() Rules {
	rootRules := []Rule{
		{`;.*$`, CommentSingle, nil},
		{`[\s]+`, Text, nil},
		{`-?\d+`, LiteralNumberInteger, nil},
		{`0x-?[abcdef\d]+`, LiteralNumberHex, nil},
		{`"(\\\\|\\"|[^"])*"`, LiteralString, nil},
		{`:[` + symChars + `]+`, LiteralStringSymbol, nil},
		{"&", Operator, nil},
	}

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{})
	for class, bindings := range hl.Classify(scope) {
		words := make([]string, len(bindings))
		for i := range bindings {
			words[i] = string(bindings[i])
		}

		tokenType, found := class2chroma[class]
		if !found {
			panic(fmt.Sprintf("unknown chroma token type for class: %s", class))
		}

		pattern := Words(`((?<![`+symChars+`])|^)`, `((?![`+symChars+`])|$)`, words...)
		rootRules = append(rootRules, Rule{
			Pattern: pattern,
			Type:    tokenType,
			Mutator: nil,
		})
	}

	rootRules = append(rootRules,
		Rule{`(?<=\()[` + symChars + `]+`, NameFunction, nil},
		Rule{`[` + symChars + `]+`, NameVariable, nil},
		Rule{`/`, Punctuation, nil},
		Rule{`(\[|\])`, Punctuation, nil},
		Rule{`(\{|\})`, Punctuation, nil},
		Rule{`(\(|\))`, Punctuation, nil})

	return Rules{
		"root": rootRules,
	}
}
