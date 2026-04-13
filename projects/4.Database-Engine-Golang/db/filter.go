// filter.go implements the query builder (Filter) and the comparison-based
// filtering engine that powers all CRUD operations against a collection.
//
// The design follows a fluent, chainable API inspired by MongoDB's query
// interface: callers build up a Filter by calling methods like Eq(), Limit(),
// and Select(), then execute a terminal operation (Find, Insert, Update, Delete)
// which opens a bbolt transaction, applies the accumulated filters, and
// returns the results.
package db

import (
	"fmt"

	"go.etcd.io/bbolt"
)

const (
	// FilterTypeEQ is the string identifier for the equality comparison filter.
	// It is used as a key in the API layer's FilterMap to route query parameters
	// like "eq.name=John" to the correct filter method.
	FilterTypeEQ = "eq"
)

// eq performs a shallow equality check between two values using Go's native
// == operator. This works correctly for comparable types (strings, numbers,
// booleans) but will panic on uncomparable types (slices, maps).
func eq(a, b any) bool {
	return a == b
}

// comparison is a function type representing a binary predicate that compares
// two values and returns whether the comparison holds. This abstraction allows
// the filter engine to support different comparison operators (eq, gt, lt, etc.)
// through a uniform interface.
type comparison func(a, b any) bool

// compFilter pairs a comparison function with a set of key-value pairs (kvs)
// that define the filter criteria. When applied to a record, it checks whether
// the record's fields satisfy the comparison for every key-value pair.
type compFilter struct {
	// kvs contains the field names and expected values to compare against.
	// For example, M{"name": "Alice", "age": 30} means the record's "name"
	// field must match "Alice" AND the "age" field must match 30.
	kvs M
	// comp is the comparison function used to evaluate each field
	// (e.g., eq for equality).
	comp comparison
}

// apply evaluates this filter against a single record. It iterates over the
// filter's key-value pairs and checks whether the record contains a matching
// value for each key using the configured comparison function.
//
// Special handling for the "id" field: since IDs are stored as uint64 but
// may arrive from the API layer as int (due to JSON number parsing), the
// value is explicitly cast to uint64 before comparison.
//
// Returns true if the record satisfies the filter, false otherwise.
func (f compFilter) apply(record M) bool {
	for k, v := range f.kvs {
		value, ok := record[k]
		if !ok {
			return false
		}
		if k == "id" {
			return f.comp(value, uint64(v.(int)))
		}
		return f.comp(value, v)
	}
	return true
}

// Filter is the query builder for a single collection. It accumulates
// comparison filters, field projections, and result limits, which are
// then applied when a terminal method (Find, Insert, Update, Delete)
// is called. Filter instances are not safe for concurrent use and should
// not be reused across queries.
type Filter struct {
	// db is a reference back to the parent database handle, used to open
	// transactions and access the configured encoder/decoder.
	db *DB
	// coll is the name of the target collection (bbolt bucket).
	coll string
	// compFilters holds the accumulated comparison filters. All filters
	// are evaluated with AND semantics — a record must pass every filter
	// to be included in the result set.
	compFilters []compFilter
	// slct holds the list of field names for projection. When non-empty,
	// only the specified fields are included in the returned documents.
	// An empty slice means "return all fields" (no projection).
	slct []string
	// limit caps the maximum number of records returned. A zero value
	// indicates no limit.
	limit int
}

// NewFilter constructs a fresh Filter targeting the given collection within
// the provided database. The returned Filter has no filters, no projection,
// and no limit — it will match all documents until further constrained.
func NewFilter(db *DB, coll string) *Filter {
	return &Filter{
		db:          db,
		coll:        coll,
		compFilters: make([]compFilter, 0),
	}
}

// Eq adds an equality filter to the query. The provided map specifies one or
// more field-value pairs; only records where every specified field equals the
// corresponding value will be included in the results.
//
// This method returns the Filter itself, enabling fluent method chaining:
//
//	db.Coll("users").Eq(db.M{"role": "admin"}).Find()
func (f *Filter) Eq(values M) *Filter {
	filt := compFilter{
		comp: eq,
		kvs:  values,
	}
	f.compFilters = append(f.compFilters, filt)
	return f
}

// Insert stores a new document in the collection. It opens a read-write
// transaction, ensures the target bucket exists (creating it lazily if needed),
// generates a monotonically increasing sequence ID, encodes the document using
// the configured Encoder, and persists it.
//
// Returns the newly assigned uint64 ID on success, or an error if any step
// in the transaction pipeline fails. The transaction is automatically rolled
// back via defer if Commit is not reached.
func (f *Filter) Insert(values M) (uint64, error) {
	tx, err := f.db.db.Begin(true)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// CreateBucketIfNotExists provides lazy collection creation — callers
	// do not need to explicitly create a collection before inserting.
	collBucket, err := tx.CreateBucketIfNotExists([]byte(f.coll))
	if err != nil {
		return 0, err
	}

	// NextSequence returns a monotonically increasing uint64 that is unique
	// within this bucket, serving as the document's primary key.
	id, err := collBucket.NextSequence()
	if err != nil {
		return 0, err
	}

	// Serialize the document and store it under the generated ID key.
	b, err := f.db.Encoder.Encode(values)
	if err != nil {
		return 0, err
	}
	if err := collBucket.Put(uint64Bytes(id), b); err != nil {
		return 0, err
	}

	return id, tx.Commit()
}

// Find retrieves all documents from the collection that match the accumulated
// filters. It opens a read-write transaction, locates the target bucket,
// iterates over every key-value pair in the bucket, and applies each registered
// comparison filter. Documents that pass all filters (AND semantics) and
// survive the optional field projection are returned as a slice of M.
//
// Returns an error if the collection does not exist or if the transaction fails.
func (f *Filter) Find() ([]M, error) {
	tx, err := f.db.db.Begin(true)
	if err != nil {
		return nil, err
	}

	bucket := tx.Bucket([]byte(f.coll))
	if bucket == nil {
		return nil, fmt.Errorf("bucket (%s) not found", f.coll)
	}

	records, err := f.findFiltered(bucket)
	fmt.Println("records", records)
	if err != nil {
		return nil, err
	}

	return records, tx.Commit()
}

// Update modifies all documents that match the accumulated filters. For each
// matching record, it merges the provided values map: only fields that already
// exist in the record are overwritten (new fields are not added). The updated
// documents are re-encoded and written back to the bucket within the same
// transaction, ensuring atomicity.
//
// Returns the slice of updated documents (post-modification) on success.
func (f *Filter) Update(values M) ([]M, error) {
	tx, err := f.db.db.Begin(true)
	if err != nil {
		return nil, err
	}

	bucket := tx.Bucket([]byte(f.coll))
	if bucket == nil {
		return nil, fmt.Errorf("bucket (%s) not found", f.coll)
	}

	// First, find all records that match the current filter criteria.
	records, err := f.findFiltered(bucket)
	if err != nil {
		return nil, err
	}

	// Apply the update: iterate over each matched record and overwrite
	// only the fields that are present in both the record and the update map.
	// This is a "patch" semantic — it preserves fields not mentioned in `values`.
	for _, record := range records {
		for k, v := range values {
			if _, ok := record[k]; ok {
				record[k] = v
			}
		}

		// Re-encode and persist the modified record under the same key.
		b, err := f.db.Encoder.Encode(record)
		if err != nil {
			return nil, err
		}
		if err := bucket.Put(uint64Bytes(record["id"].(uint64)), b); err != nil {
			return nil, err
		}
	}

	return records, tx.Commit()
}

// Delete removes all documents from the collection that match the accumulated
// filters. It finds matching records, extracts their IDs, and deletes the
// corresponding key-value pairs from the bucket within a single transaction.
//
// Returns an error if the collection does not exist, if the filter scan fails,
// or if any individual delete operation fails.
func (f *Filter) Delete() error {
	tx, err := f.db.db.Begin(true)
	if err != nil {
		return err
	}

	bucket := tx.Bucket([]byte(f.coll))
	if bucket == nil {
		return fmt.Errorf("bucket (%s) not found", f.coll)
	}

	records, err := f.findFiltered(bucket)
	if err != nil {
		return err
	}

	// Remove each matched record by its serialized uint64 key.
	for _, r := range records {
		idbytes := uint64Bytes(r["id"].(uint64))
		if err := bucket.Delete(idbytes); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Limit sets the maximum number of documents to return from a query.
// A value of 0 means no limit. Returns the Filter for method chaining.
func (f *Filter) Limit(n int) *Filter {
	f.limit = n
	return f
}

// Select specifies a field projection — only the named fields will be
// included in the returned documents. If Select is never called (or called
// with no arguments), all fields are returned. Returns the Filter for
// method chaining.
//
// Example:
//
//	db.Coll("users").Select("name", "email").Find()
func (f *Filter) Select(values ...string) *Filter {
	f.slct = append(f.slct, values...)
	return f
}

// findFiltered performs a full scan of the given bbolt bucket, deserializes
// each stored document, and applies all registered comparison filters using
// AND semantics. Only records that pass every filter are included in the
// result set. After filtering, the optional field projection (Select) is
// applied to each surviving record.
//
// The record's primary key is injected as the "id" field before filtering,
// so filters and projections can reference "id" like any other field.
//
// This is a private helper used by Find, Update, and Delete to share the
// common scan-and-filter logic.
func (f *Filter) findFiltered(bucket *bbolt.Bucket) ([]M, error) {
	results := []M{}

	bucket.ForEach(func(k, v []byte) error {
		// Reconstruct the document: start with the ID derived from the key,
		// then decode the stored value on top of it. This ensures "id" is
		// always present in the record regardless of what was originally stored.
		record := M{
			"id": uint64FromBytes(k),
		}
		if err := f.db.Decoder.Decode(v, &record); err != nil {
			return err
		}

		// Evaluate all comparison filters with short-circuit AND logic.
		// If any filter rejects the record, skip it immediately.
		include := true
		for _, filter := range f.compFilters {
			if !filter.apply(record) {
				include = false
				break
			}
		}
		if !include {
			return nil
		}

		// Apply the field projection (Select) to strip out unrequested fields.
		record = f.applySelect(record)
		results = append(results, record)
		return nil
	})

	return results, nil
}

// applySelect implements field projection. If no fields were specified via
// the Select method, the entire record is returned unmodified. Otherwise,
// a new map is constructed containing only the requested fields that exist
// in the record — unknown field names are silently ignored rather than
// causing an error.
func (f *Filter) applySelect(record M) M {
	if len(f.slct) == 0 {
		return record
	}
	data := M{}
	for _, key := range f.slct {
		if _, ok := record[key]; ok {
			data[key] = record[key]
		}
	}
	return data
}
