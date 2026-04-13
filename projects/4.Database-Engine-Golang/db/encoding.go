package db

import "encoding/json"

// DataEncoder defines the contract for serializing a document (M) into a byte
// slice suitable for storage. Implementations must produce a deterministic or
// at least decodable representation so that the corresponding DataDecoder can
// reconstruct the original document.
type DataEncoder interface {
	Encode(M) ([]byte, error)
}

// DataDecoder defines the contract for deserializing a byte slice back into
// a Go value. The second parameter is a pointer to the target value, following
// the same convention as encoding/json.Unmarshal and similar standard-library
// decoders.
type DataDecoder interface {
	Decode([]byte, any) error
}

// JSONEncoder is the default DataEncoder implementation. It delegates to
// encoding/json.Marshal, producing a compact JSON byte representation
// of the document.
type JSONEncoder struct{}

// Encode serializes the given document map into JSON bytes.
func (JSONEncoder) Encode(data M) ([]byte, error) {
	return json.Marshal(data)
}

// JSONDecoder is the default DataDecoder implementation. It delegates to
// encoding/json.Unmarshal to reconstruct a document from its JSON byte
// representation.
type JSONDecoder struct{}

// Decode deserializes the JSON byte slice into the value pointed to by v.
// The pointer indirection (&v) is intentional — it allows json.Unmarshal
// to populate the underlying map even when v is already a pointer, ensuring
// the caller's original variable is updated.
func (JSONDecoder) Decode(b []byte, v any) error {
	return json.Unmarshal(b, &v)
}
