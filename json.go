package bass

import (
	"bytes"
	"encoding/json"
)

func UnmarshalJSON(payload []byte, dest interface{}) error {
	buf := bytes.NewBuffer(payload)

	dec := json.NewDecoder(buf)
	dec.UseNumber()
	return dec.Decode(dest)
}

func MarshalJSON(val interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	err := enc.Encode(val)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
