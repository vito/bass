package bass

import (
	"bytes"
	"encoding/json"
	"io"
)

func NewRawDecoder(r io.Reader) *json.Decoder {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{NewRawDecoder(r)}
}

type Decoder struct {
	dec *json.Decoder
}

func (dec *Decoder) Decode(dest any) error {
	var any any
	err := dec.dec.Decode(&any)
	if err != nil {
		return err
	}

	val, err := ValueOf(any)
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

func RawUnmarshalJSON(payload []byte, dest any) error {
	return NewRawDecoder(bytes.NewBuffer(payload)).Decode(dest)
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
