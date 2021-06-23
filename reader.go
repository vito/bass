package bass

import (
	"fmt"
	"io"
	"strconv"

	"github.com/spy16/slurp/core"
	"github.com/spy16/slurp/reader"
)

type Reader struct {
	r *reader.Reader
}

var symTable = map[string]core.Any{
	"null":  Null{},
	"true":  Bool(true),
	"false": Bool(false),
}

func NewReader(src io.Reader) *Reader {
	r := reader.New(
		src,
		reader.WithNumReader(readInt),
		reader.WithSymbolReader(readSymbol),
	)

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
