package bass

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spy16/slurp/core"
	"github.com/spy16/slurp/reader"
)

type Reader struct {
	r *reader.Reader
}

const pairDot = Symbol(".")

var (
	symTable = map[string]core.Any{
		"_":     Ignore{},
		"null":  Null{},
		"true":  Bool(true),
		"false": Bool(false),
	}

	escapeMap = map[rune]rune{
		'"':  '"',
		'n':  '\n',
		'\\': '\\',
		't':  '\t',
		'a':  '\a',
		'f':  '\a',
		'r':  '\r',
		'b':  '\b',
		'v':  '\v',
	}
)

func NewReader(src io.Reader, name ...string) *Reader {
	r := reader.New(
		src,
		reader.WithNumReader(readInt),
		reader.WithSymbolReader(readSymbol),
	)

	if len(name) > 0 {
		r.File = name[0]
	}

	r.SetMacro('"', false, readString)
	r.SetMacro('(', false, readList)
	r.SetMacro('[', false, readConsList)
	r.SetMacro('{', false, readAssoc)
	r.SetMacro('}', false, reader.UnmatchedDelimiter())
	r.SetMacro(';', false, readCommented)
	r.SetMacro(':', false, readKeyword)
	r.SetMacro('!', true, readShebang)
	r.SetMacro('\'', false, nil)
	r.SetMacro('~', false, nil)
	r.SetMacro('`', false, nil)

	return &Reader{
		r: r,
	}
}

func (reader *Reader) Next() (Value, error) {
	return readAnnotated(reader.r)
}

func readAnnotated(rd *reader.Reader) (Annotated, error) {
	pre := rd.Position()

	any, err := rd.One()
	if err != nil {
		return Annotated{}, err
	}

	val, ok := any.(Value)
	if !ok {
		return Annotated{}, fmt.Errorf("read: expected Value, got %T", any)
	}

	annotated := Annotated{
		Value: val,

		Range: Range{
			Start: pre,
			End:   rd.Position(),
		},
	}

	annotated.Comment, _ = tryReadTrailingComment(rd)

	return annotated, nil
}

func readKeyword(rd *reader.Reader, init rune) (core.Any, error) {
	beginPos := rd.Position()

	token, err := rd.Token(-1)
	if err != nil {
		return nil, annotateErr(rd, err, beginPos, token)
	}

	return Keyword(unhyphenate(token)), nil
}

func unhyphenate(s string) string {
	return strings.Replace(s, "-", "_", -1)
}

func readAssoc(rd *reader.Reader, _ rune) (core.Any, error) {
	const assocEnd = '}'

	assoc := Assoc{}

	var haveKey bool
	pair := Pair{}
	err := container(rd, assocEnd, "Assoc", func(val core.Any) error {
		if !haveKey {
			pair.A = val.(Value)
			haveKey = true
		} else {
			pair.D = val.(Value)
			assoc = append(assoc, pair)
			pair = Pair{}
			haveKey = false
		}

		return nil
	})
	if haveKey {
		return nil, ErrBadSyntax
	}

	return assoc, err
}

func readSymbol(rd *reader.Reader, init rune) (core.Any, error) {
	beginPos := rd.Position()

	s, err := rd.Token(init)
	if err != nil {
		return nil, annotateErr(rd, err, beginPos, s)
	}

	if predefVal, found := symTable[s]; found {
		return predefVal, nil
	}

	segments := strings.Split(s, "/")
	if len(segments) > 1 {
		path, err := readPath(segments)
		if err != nil {
			return nil, annotateErr(rd, err, beginPos, s)
		}

		return path, nil
	}

	if s != "." && strings.HasPrefix(s, ".") {
		return CommandPath{strings.TrimPrefix(s, ".")}, nil
	}

	return Symbol(s), nil
}

func readPath(segments []string) (Value, error) {
	if len(segments) == 0 {
		return nil, fmt.Errorf("impossible: empty path segments")
	}

	start := segments[0]
	end := len(segments) - 1
	isDir := segments[end] == ""
	if isDir {
		end--
	}

	var path Value
	if start == "." {
		path = DirPath{
			Path: start,
		}
	} else if start == "" {
		path = DirPath{}
	} else {
		path = Symbol(start)
	}

	for i := 1; i <= end; i++ {
		var child Path
		if i == end && !isDir {
			child = FilePath{
				Path: segments[i],
			}
		} else {
			child = DirPath{
				Path: segments[i],
			}
		}

		path = ExtendPath{
			Parent: path,
			Child:  child,
		}
	}

	return path, nil
}

func readInt(rd *reader.Reader, init rune) (core.Any, error) {
	beginPos := rd.Position()

	numStr, err := rd.Token(init)
	if err != nil {
		return nil, err
	}

	v, err := strconv.ParseInt(numStr, 0, 64)
	if err != nil {
		return nil, annotateErr(rd, reader.ErrNumberFormat, beginPos, numStr)
	}

	return Int(v), nil
}

func readString(rd *reader.Reader, init rune) (core.Any, error) {
	beginPos := rd.Position()

	var b strings.Builder
	for {
		r, err := rd.NextRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = reader.ErrEOF
			}
			return nil, annotateErr(rd, err, beginPos, string(init)+b.String())
		}

		if r == '\\' {
			r2, err := rd.NextRune()
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = reader.ErrEOF
				}

				return nil, annotateErr(rd, err, beginPos, string(init)+b.String())
			}

			// TODO: Support for Unicode escape \uNN format.

			escaped, err := getEscape(r2)
			if err != nil {
				return nil, err
			}
			r = escaped
		} else if r == '"' {
			break
		}

		b.WriteRune(r)
	}

	return String(b.String()), nil
}

func getEscape(r rune) (rune, error) {
	escaped, found := escapeMap[r]
	if !found {
		return -1, fmt.Errorf("illegal escape sequence '\\%c'", r)
	}

	return escaped, nil
}

func readConsList(rd *reader.Reader, _ rune) (core.Any, error) {
	const end = ']'

	var dotted bool
	var list Value = Empty{}

	var vals []Value
	err := container(rd, end, "Cons", func(any core.Any) error {
		val := any.(Value)
		if val.Equal(pairDot) {
			dotted = true
		} else if dotted {
			list = val.(Value)
		} else {
			vals = append(vals, val.(Value))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := len(vals) - 1; i >= 0; i-- {
		list = Cons{
			A: vals[i],
			D: list,
		}
	}

	return list, nil
}

func readList(rd *reader.Reader, _ rune) (core.Any, error) {
	const end = ')'

	var dotted bool
	var list Value = Empty{}

	var vals []Value
	err := container(rd, end, "List", func(any core.Any) error {
		val := any.(Value)
		if val.Equal(pairDot) {
			dotted = true
		} else if dotted {
			list = val.(Value)
		} else {
			vals = append(vals, val.(Value))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	for i := len(vals) - 1; i >= 0; i-- {
		list = Pair{
			A: vals[i],
			D: list,
		}
	}

	return list, nil
}

func container(rd *reader.Reader, end rune, formType string, f func(core.Any) error) error {
	for {
		if err := rd.SkipSpaces(); err != nil {
			if err == io.EOF {
				return reader.Error{Cause: reader.ErrEOF}
			}
			return err
		}

		r, err := rd.NextRune()
		if err != nil {
			if err == io.EOF {
				return reader.Error{Cause: reader.ErrEOF}
			}
			return err
		}

		if r == end {
			break
		}
		rd.Unread(r)

		expr, err := readAnnotated(rd)
		if err != nil {
			if err == reader.ErrSkip {
				continue
			}
			return err
		}

		// TODO(performance):  verify `f` is inlined by the compiler
		if err = f(expr); err != nil {
			return err
		}
	}

	return nil
}

func readCommented(rd *reader.Reader, _ rune) (core.Any, error) {
	var comment []string
	var para []string

	for {
		err := skipLeadingComment(rd)
		if err != nil {
			return nil, err
		}

		line, err := readCommentedLine(rd)
		if err != nil {
			return nil, err
		}

		if line == "" {
			comment = append(comment, strings.Join(para, " "))
			para = nil
		} else {
			para = append(para, line)
		}

		err = skipLineSpaces(rd)
		if err != nil {
			return nil, err
		}

		next, err := rd.NextRune()
		if err != nil {
			return nil, err
		}

		if next != ';' {
			rd.Unread(next)
			break
		}
	}

	if len(para) > 0 {
		comment = append(comment, strings.Join(para, " "))
	}

	annotated, err := readAnnotated(rd)
	if err != nil {
		return nil, err
	}

	annotated.Comment = strings.Join(comment, "\n\n")

	return annotated, nil
}

func tryReadTrailingComment(rd *reader.Reader) (string, bool) {
	err := skipLineSpaces(rd)
	if err != nil {
		return "", false
	}

	next, err := rd.NextRune()
	if err != nil {
		return "", false
	}

	if next != ';' {
		rd.Unread(next)
		return "", false
	}

	err = skipLeadingComment(rd)
	if err != nil {
		return "", false
	}

	line, err := readCommentedLine(rd)
	if err != nil {
		return "", false
	}

	return line, true
}

func skipLeadingComment(rd *reader.Reader) error {
	for {
		next, err := rd.NextRune()
		if err != nil {
			return err
		}

		if next != ';' {
			rd.Unread(next)
			break
		}
	}

	return skipLineSpaces(rd)
}

func skipLineSpaces(rd *reader.Reader) error {
	for {
		next, err := rd.NextRune()
		if err != nil {
			return err
		}

		if next != ' ' {
			rd.Unread(next)
			break
		}
	}

	return nil
}

func readCommentedLine(rd *reader.Reader) (string, error) {
	var line string
	for {
		r, err := rd.NextRune()
		if err != nil {
			if err == io.EOF {
				return line, nil
			}

			return "", err
		}

		if r == '\n' {
			break
		}

		line += string(r)
	}

	return line, nil
}

func readShebang(rd *reader.Reader, _ rune) (core.Any, error) {
	for {
		r, err := rd.NextRune()
		if err != nil {
			return nil, err
		}

		if r == '\n' {
			break
		}
	}

	return nil, reader.ErrSkip
}

func annotateErr(rd *reader.Reader, err error, beginPos reader.Position, form string) error {
	if err == io.EOF || err == reader.ErrSkip {
		return err
	}

	readErr := reader.Error{}
	if e, ok := err.(reader.Error); ok {
		readErr = e
	} else {
		readErr = reader.Error{Cause: err}
	}

	readErr.Form = form
	readErr.Begin = beginPos
	readErr.End = rd.Position()
	return readErr
}
