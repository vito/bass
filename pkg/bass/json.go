package bass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{NewRawDecoder(r)}
}

func NewRawDecoder(r io.Reader) *json.Decoder {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec
}

type Decoder struct {
	dec *json.Decoder
}

func (dec *Decoder) Decode(dest any) error {
	val, err := decodeValue(dec.dec)
	if err != nil {
		return err
	}

	return val.Decode(dest)
}

func NewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func UnmarshalJSON(payload []byte, dest any) error {
	return NewDecoder(bytes.NewBuffer(payload)).Decode(dest)
}

func MarshalJSON(val any) ([]byte, error) {
	buf := new(bytes.Buffer)

	err := NewEncoder(buf).Encode(val)
	if err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}

func decodeValue(dec *json.Decoder) (Value, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch x := tok.(type) {
	case nil:
		return Null{}, nil
	case bool:
		return Bool(x), nil
	case string:
		return String(x), nil

	case json.Number:
		i, err := x.Int64()
		if err != nil {
			// TODO: make sure there's a test or justification for this, just
			// matching current impl for now
			return String(x.String()), nil
		} else {
			return Int(i), nil
		}

	case json.Delim:
		switch x {
		case '{':
			scope := NewEmptyScope()

			for dec.More() {
				key, err := dec.Token()
				if err != nil {
					return nil, err
				}

				str, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("expected string key, got %T", key)
				}

				sym := SymbolFromJSONKey(str)

				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}

				scope.Set(sym, val)
			}

			end, err := dec.Token()
			if err != nil {
				return nil, err
			}

			if end != json.Delim('}') {
				return nil, fmt.Errorf("expected end of object, got %T: %v", end, end)
			}

			return scope, nil
		case '[':
			var vals []Value
			for dec.More() {
				val, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}

				vals = append(vals, val)
			}

			end, err := dec.Token()
			if err != nil {
				return nil, err
			}

			if end != json.Delim(']') {
				return nil, fmt.Errorf("expected end of array, got %T: %v", end, end)
			}

			return NewList(vals...), nil
		default:
			return nil, fmt.Errorf("impossible: unknown delimiter: %s", x)
		}
	default:
		return nil, fmt.Errorf("impossible: unknown delimiter: %s", x)
	}
}
