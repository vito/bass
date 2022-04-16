package bass

import (
	"bytes"
	"encoding/json"
	"io"
)

func NewDecoder(r io.Reader) *Decoder {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return &Decoder{dec}
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
