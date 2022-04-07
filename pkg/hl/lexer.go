package hl

import (
	"fmt"

	"github.com/alecthomas/chroma"
	. "github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/vito/bass/pkg/bass"
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

var class2chroma = map[Class]chroma.TokenType{
	Bool:    chroma.KeywordConstant,
	Const:   chroma.KeywordConstant,
	Cond:    chroma.Keyword,
	Repeat:  chroma.NameBuiltin,
	Var:     chroma.NameBuiltinPseudo,
	Def:     chroma.KeywordDeclaration,
	Fn:      chroma.NameFunction,
	Op:      chroma.NameBuiltin,
	Special: chroma.Keyword,
}

// taken from chroma's TTY formatter
var ttyMap = map[string]string{
	"30m": "#000000", "31m": "#7f0000", "32m": "#007f00", "33m": "#7f7fe0",
	"34m": "#00007f", "35m": "#7f007f", "36m": "#007f7f", "37m": "#e5e5e5",
	"90m": "#555555", "91m": "#ff0000", "92m": "#00ff00", "93m": "#ffff00",
	"94m": "#0000ff", "95m": "#ff00ff", "96m": "#00ffff", "97m": "#ffffff",
}

// TTY style matches to hex codes used by the TTY formatter to map them to
// specific ANSI escape codes.
var TTYStyle = styles.Register(chroma.MustNewStyle("tty", chroma.StyleEntries{
	chroma.Comment:             ttyMap["95m"] + " italic",
	chroma.CommentPreproc:      ttyMap["90m"],
	chroma.KeywordConstant:     ttyMap["33m"],
	chroma.Keyword:             ttyMap["31m"],
	chroma.KeywordDeclaration:  ttyMap["35m"],
	chroma.NameBuiltin:         ttyMap["31m"],
	chroma.NameBuiltinPseudo:   ttyMap["36m"],
	chroma.NameFunction:        ttyMap["34m"],
	chroma.NameNamespace:       ttyMap["34m"],
	chroma.LiteralNumber:       ttyMap["31m"],
	chroma.LiteralString:       ttyMap["32m"],
	chroma.LiteralStringSymbol: ttyMap["33m"],
	chroma.Operator:            ttyMap["31m"],
	chroma.Punctuation:         ttyMap["90m"],
}))

const symChars = `\w!$%*+<=>?.#\-`

func bassRules() Rules {
	rootRules := []Rule{
		{`^#!.*$`, CommentPreproc, nil},
		{`;.*$`, CommentSingle, nil},
		{`[\s]+`, Text, nil},
		{`-?\d+`, LiteralNumberInteger, nil},
		{`0x-?[abcdef\d]+`, LiteralNumberHex, nil},
		{`"(\\\\|\\"|[^"])*"`, LiteralString, nil},
		{`:[` + symChars + `]+`, LiteralStringSymbol, nil},
		{"&", Operator, nil},
	}

	scope := runtimes.NewScope(bass.Ground, runtimes.RunState{})
	for _, class := range Classify(scope) {
		words := make([]string, len(class.Bindings))
		for i := range class.Bindings {
			words[i] = string(class.Bindings[i])
		}

		tokenType, found := class2chroma[class.Class]
		if !found {
			panic(fmt.Sprintf("unknown chroma token type for class: %s", class))
		}

		pattern := Words(`((?<![`+symChars+`/])|^)`, `((?![`+symChars+`])|$)`, words...)
		rootRules = append(rootRules, Rule{
			Pattern: pattern,
			Type:    tokenType,
			Mutator: nil,
		})
	}

	rootRules = append(rootRules,
		Rule{`(?<=\()[` + symChars + `]+`, NameFunction, nil},
		Rule{`[` + symChars + `]+`, NameVariable, nil},
		Rule{`/`, NameFunction, nil},
		Rule{`(\[|\])`, Punctuation, nil},
		Rule{`(\{|\})`, Punctuation, nil},
		Rule{`(\(|\))`, Punctuation, nil})

	return Rules{
		"root": rootRules,
	}
}
