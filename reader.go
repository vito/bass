package bass

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	slurpcore "github.com/spy16/slurp/core"
	slurpreader "github.com/spy16/slurp/reader"
)

type Reader struct {
	rd *slurpreader.Reader

	Analyzer FormAnalyzer
	Context  context.Context
}

type FormAnalyzer interface {
	Analyze(context.Context, Annotated)
}

const pairDelim = Symbol("&")

var (
	symTable = map[string]slurpcore.Any{
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
	r := slurpreader.New(
		src,
		slurpreader.WithNumReader(readInt),
		slurpreader.WithSymbolReader(readSymbol),
	)

	if len(name) > 0 {
		r.File = name[0]
	}

	reader := &Reader{
		rd: r,
	}

	r.SetMacro('"', false, readString)
	r.SetMacro('(', false, reader.readList)
	r.SetMacro('[', false, reader.readConsList)
	r.SetMacro('{', false, reader.readBind)
	r.SetMacro('}', false, slurpreader.UnmatchedDelimiter())
	r.SetMacro(';', false, reader.readCommented)
	r.SetMacro('!', true, readShebang)
	r.SetMacro('\'', false, nil)
	r.SetMacro('~', false, nil)
	r.SetMacro('`', false, nil)
	r.SetMacro(':', false, nil)

	return reader
}

func (reader *Reader) Next() (Value, error) {
	return reader.readAnnotated()
}

func (reader *Reader) readAnnotated() (Annotated, error) {
	rd := reader.rd

	if err := rd.SkipSpaces(); err != nil {
		return Annotated{}, err
	}

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

	if reader.Analyzer != nil {
		reader.Analyzer.Analyze(reader.Context, annotated)
	}

	return annotated, nil
}

func (reader *Reader) readBind(rd *slurpreader.Reader, _ rune) (slurpcore.Any, error) {
	const assocEnd = '}'

	bind := Bind{}

	err := reader.container(assocEnd, "Bind", func(any slurpcore.Any) error {
		bind = append(bind, any.(Value))
		return nil
	})

	return bind, err
}

func readSymbol(rd *slurpreader.Reader, init rune) (slurpcore.Any, error) {
	beginPos := rd.Position()

	s, err := rd.Token(init)
	if err != nil {
		return nil, annotateErr(rd, err, beginPos, s)
	}

	if predefVal, found := symTable[s]; found {
		return predefVal, nil
	}

	pathSegments := strings.Split(s, "/")
	if len(pathSegments) > 1 {
		path, err := readPath(pathSegments)
		if err != nil {
			return nil, annotateErr(rd, err, beginPos, s)
		}

		return path, nil
	}

	if s != "." && strings.HasPrefix(s, ".") {
		return CommandPath{strings.TrimPrefix(s, ".")}, nil
	}

	val, err := readKeywordsOrJustSymbol(s)
	if err != nil {
		return nil, annotateErr(rd, err, beginPos, s)
	}

	return val, nil
}

func readKeywordsOrJustSymbol(s string) (Value, error) {
	kwSegments := strings.Split(s, ":")
	if len(kwSegments) == 1 {
		return Symbol(s), nil
	}

	val, err := readKeywords(kwSegments)
	if err != nil {
		return nil, err
	}

	return val, nil
}

func readKeywords(segments []string) (Value, error) {
	start := segments[0]

	begin := 1

	var val Value

	isKeyword := start == ""
	if isKeyword {
		val = Keyword(segments[1])
		begin++
	} else {
		val = Symbol(start)
	}

	for i := begin; i <= len(segments)-1; i++ {
		val = NewList(Keyword(segments[i]), val)
	}

	return val, nil
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
		var err error
		path, err = readKeywordsOrJustSymbol(start)
		if err != nil {
			return nil, err
		}
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

func readInt(rd *slurpreader.Reader, init rune) (slurpcore.Any, error) {
	beginPos := rd.Position()

	numStr, err := rd.Token(init)
	if err != nil {
		return nil, err
	}

	v, err := strconv.ParseInt(numStr, 0, 64)
	if err != nil {
		return nil, annotateErr(rd, slurpreader.ErrNumberFormat, beginPos, numStr)
	}

	return Int(v), nil
}

func readString(rd *slurpreader.Reader, init rune) (slurpcore.Any, error) {
	beginPos := rd.Position()

	var b strings.Builder
	for {
		r, err := rd.NextRune()
		if err != nil {
			if errors.Is(err, io.EOF) {
				err = slurpreader.ErrEOF
			}
			return nil, annotateErr(rd, err, beginPos, string(init)+b.String())
		}

		if r == '\\' {
			r2, err := rd.NextRune()
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = slurpreader.ErrEOF
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

func (reader *Reader) readConsList(rd *slurpreader.Reader, _ rune) (slurpcore.Any, error) {
	const end = ']'

	var dotted bool
	var list Value = Empty{}

	var vals []Value
	err := reader.container(end, "Cons", func(any slurpcore.Any) error {
		val := any.(Value)
		if val.Equal(pairDelim) {
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

func (reader *Reader) readList(rd *slurpreader.Reader, _ rune) (slurpcore.Any, error) {
	const end = ')'

	var dotted bool
	var list Value = Empty{}

	var vals []Value
	err := reader.container(end, "List", func(any slurpcore.Any) error {
		val := any.(Value)
		if val.Equal(pairDelim) {
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

func (reader *Reader) container(end rune, _ string, f func(slurpcore.Any) error) error {
	rd := reader.rd

	for {
		if err := rd.SkipSpaces(); err != nil {
			if err == io.EOF {
				return slurpreader.Error{Cause: slurpreader.ErrEOF}
			}
			return err
		}

		r, err := rd.NextRune()
		if err != nil {
			if err == io.EOF {
				return slurpreader.Error{Cause: slurpreader.ErrEOF}
			}
			return err
		}

		if r == end {
			break
		}
		rd.Unread(r)

		expr, err := reader.readAnnotated()
		if err != nil {
			if err == slurpreader.ErrSkip {
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

func (reader *Reader) readCommented(rd *slurpreader.Reader, _ rune) (slurpcore.Any, error) {
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

	annotated, err := reader.readAnnotated()
	if err != nil {
		return nil, err
	}

	annotated.Comment = strings.Join(comment, "\n\n")

	return annotated, nil
}

func tryReadTrailingComment(rd *slurpreader.Reader) (string, bool) {
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

func skipLeadingComment(rd *slurpreader.Reader) error {
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

func skipLineSpaces(rd *slurpreader.Reader) error {
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

func readCommentedLine(rd *slurpreader.Reader) (string, error) {
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

func readShebang(rd *slurpreader.Reader, _ rune) (slurpcore.Any, error) {
	for {
		r, err := rd.NextRune()
		if err != nil {
			return nil, err
		}

		if r == '\n' {
			break
		}
	}

	return nil, slurpreader.ErrSkip
}

func annotateErr(rd *slurpreader.Reader, err error, beginPos slurpreader.Position, form string) error {
	if err == io.EOF || err == slurpreader.ErrSkip {
		return err
	}

	readErr := slurpreader.Error{}
	if e, ok := err.(slurpreader.Error); ok {
		readErr = e
	} else {
		readErr = slurpreader.Error{Cause: err}
	}

	readErr.Form = form
	readErr.Begin = beginPos
	readErr.End = rd.Position()
	return readErr
}
