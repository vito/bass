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

func NewReader(src io.Reader) *Reader {
	r := reader.New(
		src,
		reader.WithNumReader(readInt),
		reader.WithSymbolReader(readSymbol),
	)

	r.SetMacro('"', false, readString)
	r.SetMacro('(', false, readList)
	r.SetMacro('[', false, readConsList)
	r.SetMacro(';', false, readCommented)

	return &Reader{
		r: r,
	}
}

func (reader *Reader) Next() (Value, error) {
	return readOne(reader.r)
}

func readOne(rd *reader.Reader) (Value, error) {
	pre := rd.Position()

	any, err := rd.One()
	if err != nil {
		return nil, err
	}

	val, ok := any.(Value)
	if !ok {
		return nil, fmt.Errorf("read: expected Value, got %T", any)
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

func readSymbol(rd *reader.Reader, init rune) (core.Any, error) {
	beginPos := rd.Position()

	s, err := rd.Token(init)
	if err != nil {
		return nil, annotateErr(rd, err, beginPos, s)
	}

	if predefVal, found := symTable[s]; found {
		return predefVal, nil
	}

	return Symbol(s), nil
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

		expr, err := readOne(rd)
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

	val, err := readOne(rd)
	if err != nil {
		return nil, err
	}

	return Annotated{
		Comment: strings.Join(comment, "\n\n"),
		Value:   val,
	}, nil
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
