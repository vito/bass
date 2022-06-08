package bass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{newRawDecoder(r)}
}

func newRawDecoder(r io.Reader) *json.Decoder {
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

// Descope is a funky little function that decodes from the scope form of a
// first-class value type, like a thunk path or file path. This typically
// happens when encoding to and from JSON.
func Descope(val Value) Value {
	var thunk Thunk
	if err := val.Decode(&thunk); err == nil {
		return thunk
	}

	var file FilePath
	if err := val.Decode(&file); err == nil {
		return file
	}

	var dir DirPath
	if err := val.Decode(&dir); err == nil {
		return dir
	}

	var cmdp CommandPath
	if err := val.Decode(&cmdp); err == nil {
		return cmdp
	}

	var thunkPath ThunkPath
	if err := val.Decode(&thunkPath); err == nil {
		return thunkPath
	}

	var host HostPath
	if err := val.Decode(&host); err == nil {
		return host
	}

	var secret Secret
	if err := val.Decode(&secret); err == nil {
		return secret
	}

	return val
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

			return Descope(scope), nil
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
