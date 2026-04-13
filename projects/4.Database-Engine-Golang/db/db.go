// Package db implements a document-oriented database engine on top of bbolt,
// an embedded key-value store. It provides a MongoDB-inspired API with
// collections, chainable query filters, and pluggable serialization.
//
// Architecture overview:
//   - DB is the top-level handle to the underlying bbolt database file.
//   - Collections are represented as bbolt buckets, each storing documents
//     as binary-encoded key-value pairs where the key is an auto-incrementing
//     uint64 sequence ID and the value is the encoded document.
//   - All reads and writes go through bbolt transactions, ensuring ACID
//     guarantees at the storage layer.
//   - Encoding and decoding of documents is abstracted behind the DataEncoder
//     and DataDecoder interfaces, allowing callers to swap JSON for any other
//     format (e.g., MessagePack, Protocol Buffers) via functional options.
package db

import (
	"fmt"
	"os"

	"go.etcd.io/bbolt"
)

const (
	// defaultDBName is the base filename used when no custom name is supplied.
	defaultDBName = "database"
	// ext is the file extension appended to the database filename.
	ext = "db"
)

// M is a shorthand type alias for a generic document map. It mirrors the
// pattern used by MongoDB drivers (bson.M) to represent schema-free documents
// as string-keyed maps of arbitrary values.
type M map[string]any

// DB is the primary database handle. It wraps a bbolt.DB instance and carries
// the active configuration (encoding strategy, database name) that was
// resolved at construction time through the functional options pattern.
type DB struct {
	// currentDatabase holds the resolved filename (e.g., "database.db")
	// of the bbolt file that is currently open.
	currentDatabase string
	// Options is embedded so that encoder/decoder and other settings are
	// directly accessible from the DB struct without an extra field dereference.
	*Options
	// db is the underlying bbolt database handle used for all storage operations.
	db *bbolt.DB
}

// New constructs a new DB instance. It accepts zero or more functional option
// functions (OptFunc) that override the default configuration. The defaults are:
//   - Database name: "database"
//   - Encoder:       JSONEncoder (encoding/json)
//   - Decoder:       JSONDecoder (encoding/json)
//
// The function opens (or creates) the bbolt file on disk and returns a ready-to-use
// DB handle. The caller is responsible for closing the database when done.
func New(options ...OptFunc) (*DB, error) {

	// Start with sensible defaults; callers override via WithDBName, WithEncoder, etc.
	opts := &Options{
		DBName:  defaultDBName,
		Encoder: JSONEncoder{},
		Decoder: JSONDecoder{},
	}

	// Apply each caller-supplied option function in order, allowing later
	// options to override earlier ones.
	for _, fn := range options {
		fn(opts)
	}

	// Construct the on-disk filename and open the bbolt database.
	// File mode 0666 allows read/write for owner, group, and others;
	// bbolt internally applies an exclusive file lock to prevent concurrent access.
	dbname := fmt.Sprintf("%s.%s", defaultDBName, ext)
	db, err := bbolt.Open(dbname, 0666, nil)
	if err != nil {
		return nil, err
	}

	return &DB{currentDatabase: dbname, db: db, Options: opts}, nil
}

// Drop removes the database file identified by the given name from disk.
// This is a destructive, irreversible operation — all data within the
// target database will be permanently deleted.
func (db *DB) Drop(name string) error {
	dbname := fmt.Sprintf("%s.%s", name, ext)
	return os.Remove(dbname)
}

// CreateCollection explicitly creates a collection (bbolt bucket) with the
// given name. If the collection already exists, this is a no-op thanks to
// bbolt's CreateBucketIfNotExists semantics. The returned bucket reference
// is bound to the transaction that created it and should not be retained
// beyond the scope of that transaction.
func (db *DB) CreateCollection(name string) (*bbolt.Bucket, error) {
	tx, err := db.db.Begin(true)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	bucket, err := tx.CreateBucketIfNotExists([]byte(name))
	if err != nil {
		return nil, err
	}

	return bucket, nil
}

// Coll returns a new Filter query builder targeting the named collection.
// This is the primary entry point for performing CRUD operations; callers
// chain filter methods (e.g., Eq, Limit, Select) before executing a
// terminal operation (Insert, Find, Update, Delete).
//
// Example:
//
//	results, err := db.Coll("users").Eq(db.M{"age": 30}).Limit(10).Find()
func (h *DB) Coll(name string) *Filter {
	return NewFilter(h, name)
}
