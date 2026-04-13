package api

import (
	"strconv"

	"github.com/YpatiosCh/Database/db"
)

// FilterMap is an intermediate data structure that collects parsed query
// parameter filters, grouped by filter type (e.g., "eq"). It acts as a
// bridge between the raw HTTP query string and the database's Filter query
// builder, handling type coercion from string-based query values into their
// appropriate Go types.
type FilterMap struct {
	// filters maps each supported filter type (e.g., "eq") to a document map
	// (db.M) containing the accumulated field-value pairs for that filter type.
	filters map[string]db.M
}

// NewFilterMap constructs a FilterMap pre-initialized with empty maps for
// all known filter types. Currently only the equality filter ("eq") is
// registered. Adding support for new comparison operators (e.g., "gt", "lt")
// requires registering them here and implementing the corresponding methods
// in the Filter query builder.
func NewFilterMap() *FilterMap {
	filters := make(map[string]db.M)
	filters[db.FilterTypeEQ] = db.M{}
	return &FilterMap{
		filters: filters,
	}
}

// Get returns the accumulated field-value pairs for the given filter type.
// If the filter type is not recognized, an empty map is returned, which
// effectively becomes a no-op when passed to the query builder.
func (f *FilterMap) Get(filterType string) db.M {
	val, ok := f.filters[filterType]
	if !ok {
		return db.M{}
	}
	return val
}

// Add registers a single filter criterion. The string value v is automatically
// coerced into its most specific Go type (bool, int, float64, or string) via
// ensureCorrectTypeFromString before being stored. If the filter type is not
// recognized, the entry is silently dropped.
func (f *FilterMap) Add(filterType, k string, v string) {
	if _, ok := f.filters[filterType]; !ok {
		return
	}
	f.filters[filterType][k] = ensureCorrectTypeFromString(v)
}

// ensureCorrectTypeFromString performs best-effort type inference on a raw
// query parameter string value. Since HTTP query parameters are always strings,
// this function attempts to parse the value into a more specific Go type so
// that downstream equality comparisons work correctly against the typed values
// stored in the database.
//
// The parsing priority is:
//  1. Boolean — exact matches "true" / "false"
//  2. Integer — valid base-10 integer (checked before float to avoid
//     "42" being parsed as float64(42.0))
//  3. Float   — valid floating-point number
//  4. String  — fallback; the original string is returned as-is
func ensureCorrectTypeFromString(v string) any {
	switch {
	case v == "true":
		return true
	case v == "false":
		return false
	case isInteger(v):
		val, _ := strconv.Atoi(v)
		return val
	case isFloat(v):
		val, _ := strconv.ParseFloat(v, 64)
		return val
	default:
		return v
	}
}

// isFloat reports whether the string s can be parsed as a 64-bit floating-point
// number. Note: integers also parse as valid floats, so this should be checked
// after isInteger to maintain the correct type inference priority.
func isFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// isInteger reports whether the string s can be parsed as a base-10 integer
// that fits within the platform's int size (strconv.Atoi).
func isInteger(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
