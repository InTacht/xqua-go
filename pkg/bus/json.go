package bus

import "encoding/json"

// EncodeJSON marshals v to JSON bytes for use as a Message payload.
func EncodeJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// DecodeJSON unmarshals a Message payload into v.
func DecodeJSON(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
