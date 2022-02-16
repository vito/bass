package bass

import (
	"bytes"
	"encoding/json"
	"io"
)

func NewDecoder(r io.Reader) *json.Decoder {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec
}

func NewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func UnmarshalJSON(payload []byte, dest interface{}) error {
	return NewDecoder(bytes.NewBuffer(payload)).Decode(dest)
}

func MarshalJSON(val interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)

	err := NewEncoder(buf).Encode(val)
	if err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte{'\n'}), nil
}
