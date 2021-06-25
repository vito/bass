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
	r.SetMacro('[', false, readInertList)

	return &Reader{
		r: r,
	}
}

func (reader *Reader) Next() (Value, error) {
	any, err := reader.r.One()
	if err != nil {
		return nil, err
	}

	val, ok := any.(Value)
	if !ok {
		return nil, fmt.Errorf("read: expected Value, got %T", any)
	}

	return val, nil
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

func readInertList(rd *reader.Reader, _ rune) (core.Any, error) {
	const end = ']'

	var dotted bool
	var list Value = Empty{}

	var vals []Value
	err := rd.Container(end, "InertList", func(val core.Any) error {
		if val == Symbol(".") {
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
		list = InertPair{
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
	err := rd.Container(end, "List", func(val core.Any) error {
		if val == Symbol(".") {
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
