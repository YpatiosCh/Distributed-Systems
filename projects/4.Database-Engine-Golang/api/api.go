// Package api implements the HTTP REST interface for the document database.
// It translates incoming HTTP requests into database operations and returns
// results as JSON. The API is designed around a resource-per-collection model
// where the collection name is extracted from the URL path.
//
// Endpoints:
//
//	POST /api/:collname         Insert a new document into the named collection.
//	GET  /api/:collname         Query documents using filter query parameters.
//
// Query parameter format for GET requests:
//
//	?<filterType>.<fieldName>=<value>
//
// For example:
//
//	GET /api/users?eq.name=Alice&eq.age=30
//
// This queries the "users" collection for documents where name == "Alice"
// AND age == 30.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/YpatiosCh/Database/db"
	"github.com/labstack/echo/v5"
)

// Server is the HTTP handler layer. It holds a reference to the database
// and exposes handler methods that conform to Echo's handler signature.
type Server struct {
	// db is the injected database dependency used by all handlers.
	db *db.DB
}

// NewServer constructs a Server with the given database handle.
// This follows the dependency injection pattern — the caller controls
// the database's lifecycle and configuration.
func NewServer(db *db.DB) *Server {
	return &Server{
		db: db,
	}
}

// HandlePostInsert handles POST /api/:collname requests. It reads a JSON
// document from the request body, inserts it into the collection identified
// by the :collname URL parameter, and returns the auto-generated document ID
// with a 201 Created status.
//
// The request body must be a valid JSON object. Any decoding error is
// propagated to Echo's error handler.
func (s *Server) HandlePostInsert(c *echo.Context) error {
	collname := c.Param("collname")
	var data db.M

	// Decode the request body directly from the stream (without buffering
	// the entire body into memory first) into a generic document map.
	if err := json.NewDecoder(c.Request().Body).Decode(&data); err != nil {
		return err
	}

	id, err := s.db.Coll(collname).Insert(data)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, db.M{"id": id})
}

// HandleGetQuery handles GET /api/:collname requests. It parses filter
// criteria from query parameters, builds a filtered query against the named
// collection, and returns all matching documents as a JSON array.
//
// Query parameters follow the convention "<filterType>.<fieldName>=<value>".
// The filterType selects the comparison operator (currently only "eq" for
// equality), and fieldName identifies the document field to compare against.
// Multiple query parameters are combined with AND semantics.
//
// Example request:
//
//	GET /api/users?eq.role=admin&eq.active=true
//
// Returns all users where role == "admin" AND active == true.
func (s *Server) HandleGetQuery(c *echo.Context) error {
	var (
		collname  = c.Param("collname")
		filterMap = NewFilterMap()
	)

	// Parse and validate each query parameter, splitting on the "." separator
	// to extract the filter type and field name.
	for k, v := range c.QueryParams() {
		filterParts := strings.Split(k, ".")
		if len(filterParts) != 2 {
			return fmt.Errorf("mallformed query")
		}
		if len(v) == 0 {
			return fmt.Errorf("mallformed query")
		}
		if v[0] == "" {
			return fmt.Errorf("mallformed query")
		}

		var (
			filterType  = filterParts[0] // e.g., "eq"
			filterKey   = filterParts[1] // e.g., "name"
			filterValue = v[0]           // e.g., "Alice" (first value only; multi-value is ignored)
		)
		filterMap.Add(filterType, filterKey, filterValue)
	}

	// Build and execute the query: extract the equality filters from the
	// filter map, attach them to the collection's query builder, and run Find.
	records, err := s.db.Coll(collname).
		Eq(filterMap.Get(db.FilterTypeEQ)).
		Find()
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, records)
}
