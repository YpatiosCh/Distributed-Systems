package db

// Options holds the configurable settings for a DB instance. It is populated
// at construction time through the functional options pattern, which allows
// callers to selectively override only the settings they care about while
// keeping sensible defaults for everything else.
type Options struct {
	// DBName is the logical name of the database. It is used as the base
	// component of the on-disk filename (e.g., "mydb" → "mydb.db").
	DBName string
	// Encoder is the strategy used to serialize documents (M) into bytes
	// before they are stored in bbolt. Defaults to JSONEncoder.
	Encoder DataEncoder
	// Decoder is the strategy used to deserialize bytes read from bbolt
	// back into document maps. Defaults to JSONDecoder.
	Decoder DataDecoder
}

// OptFunc is the signature for a functional option. Each OptFunc receives a
// mutable pointer to Options and modifies it in place. This pattern, known
// as "functional options", was popularized by Dave Cheney and Rob Pike and
// is idiomatic in Go for constructors that need flexible, extensible configuration
// without breaking backward compatibility when new settings are added.
type OptFunc func(opts *Options)

// WithDBName returns an OptFunc that overrides the default database name.
func WithDBName(name string) OptFunc {
	return func(opts *Options) {
		opts.DBName = name
	}
}

// WithDecoder returns an OptFunc that overrides the default document decoder.
// Use this to plug in a custom deserialization strategy (e.g., MessagePack).
func WithDecoder(dec DataDecoder) OptFunc {
	return func(opts *Options) {
		opts.Decoder = dec
	}
}

// WithEncoder returns an OptFunc that overrides the default document encoder.
// Use this to plug in a custom serialization strategy (e.g., MessagePack).
func WithEncoder(enc DataEncoder) OptFunc {
	return func(opts *Options) {
		opts.Encoder = enc
	}
}
