package store

// LocalDB represents a local database in the distributed key-value store.
// It contains a slice of Store, which holds key-value pairs.
type LocalDB struct {
	Store []Store
}

// Store represents a key-value pair in the distributed key-value store.
// It contains a key and its corresponding value.
type Store struct {
	Key   any
	Value any
}

// GetKey returns the key of the Store.
func (s *Store) GetKey() any {
	return s.Key
}

// GetValue returns the value of the Store.
func (s *Store) GetValue() any {
	return s.Value
}

// SetKey sets the key of the Store.
func (s *Store) SetKey(key any) {
	s.Key = key
}

// SetValue sets the value of the Store.
func (s *Store) SetValue(value any) {
	s.Value = value
}
