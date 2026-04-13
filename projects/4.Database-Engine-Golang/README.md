# Database-Golang

A lightweight, embeddable document-oriented database engine written in Go. Built on top of [bbolt](https://github.com/etcd-io/bbolt) (an embedded key-value store), it provides a MongoDB-inspired API for storing, querying, updating, and deleting schema-free JSON documents — all exposed through a clean REST interface.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Getting Started](#getting-started)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
  - [Running the Server](#running-the-server)
- [REST API Reference](#rest-api-reference)
  - [Insert a Document](#insert-a-document)
  - [Query Documents](#query-documents)
- [Query Parameter Syntax](#query-parameter-syntax)
  - [Equality Filters](#equality-filters)
  - [Type Coercion](#type-coercion)
- [Go SDK Usage](#go-sdk-usage)
  - [Initializing the Database](#initializing-the-database)
  - [Functional Options](#functional-options)
  - [Insert](#insert)
  - [Find](#find)
  - [Update](#update)
  - [Delete](#delete)
  - [Select (Field Projection)](#select-field-projection)
  - [Chaining Filters](#chaining-filters)
- [Custom Encoding](#custom-encoding)
- [Project Structure](#project-structure)
- [Configuration](#configuration)
- [License](#license)

---

## Overview

Database-Golang is a zero-dependency (beyond its Go modules), single-binary document database that:

- **Stores schema-free documents** as JSON in collections (analogous to MongoDB collections or SQL tables).
- **Auto-generates unique IDs** for every inserted document using bbolt's monotonically increasing sequence.
- **Supports filtered queries** with equality matching, field projection, and result limiting.
- **Exposes a REST API** so any application — regardless of language — can interact with the database over HTTP.
- **Offers a Go SDK** for applications that want to embed the database directly without the HTTP overhead.
- **Uses pluggable encoding** so JSON can be swapped for any serialization format (MessagePack, Protocol Buffers, etc.).

All reads and writes go through bbolt transactions, which provide full **ACID guarantees** at the storage layer.

---

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Client (any language)              │
│              HTTP requests (JSON body)               │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│                   api (HTTP Layer)                   │
│  Echo router · request parsing · type coercion      │
│  FilterMap translates query params → filter criteria │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│              db (Query Builder & Engine)             │
│  Filter: chainable Eq() · Select() · Limit()        │
│  Terminal ops: Insert · Find · Update · Delete       │
│  Pluggable DataEncoder / DataDecoder interfaces      │
└──────────────────────┬──────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│                  bbolt (Storage)                     │
│  Embedded B+ tree key-value store                    │
│  Buckets = collections · ACID transactions           │
│  Single-file on-disk persistence (database.db)       │
└─────────────────────────────────────────────────────┘
```

**Key design decisions:**

- **Collections as bbolt buckets.** Each collection maps directly to a bbolt bucket. Buckets are created lazily on first insert, so there is no need to pre-define a schema.
- **Document IDs as uint64 keys.** Every document receives an auto-incrementing uint64 ID from bbolt's bucket sequence. IDs are encoded as 8-byte little-endian byte slices for storage.
- **Functional options pattern.** The database constructor (`db.New`) accepts optional configuration functions, making it easy to override defaults without breaking backward compatibility.
- **Fluent query builder.** The `Filter` type provides a chainable API (`Eq → Select → Limit → Find`) that separates query construction from execution.

---

## Getting Started

### Prerequisites

- **Go 1.25+** (as specified in `go.mod`)

### Installation

```bash
git clone https://github.com/YpatiosCh/Database.git
cd Database
go mod download
```

### Running the Server

Using the Makefile:

```bash
# Build and run
make run-build

# Or run directly without a binary
make run
```

Or manually:

```bash
go run cmd/main.go
```

The server starts on **port 8080** by default. You will see Echo's startup banner in the terminal once it is ready to accept connections.

---

## REST API Reference

**Base URL:** `http://localhost:8080`

All request and response bodies use `Content-Type: application/json`.

### Insert a Document

**`POST /api/:collname`**

Inserts a new document into the specified collection. The collection is created automatically if it does not already exist.

**Path Parameters:**

| Parameter   | Type   | Description                          |
|-------------|--------|--------------------------------------|
| `collname`  | string | Name of the target collection        |

**Request Body:** Any valid JSON object.

**Example:**

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Alice", "age": 30, "role": "admin"}'
```

**Response (201 Created):**

```json
{
  "id": 1
}
```

**Insert more documents:**

```bash
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Bob", "age": 25, "role": "user"}'

curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Charlie", "age": 35, "role": "admin"}'
```

---

### Query Documents

**`GET /api/:collname`**

Retrieves documents from the specified collection, optionally filtered by query parameters.

**Path Parameters:**

| Parameter   | Type   | Description                          |
|-------------|--------|--------------------------------------|
| `collname`  | string | Name of the target collection        |

**Query Parameters:** See [Query Parameter Syntax](#query-parameter-syntax).

**Example — fetch all users:**

```bash
curl http://localhost:8080/api/users
```

**Response (200 OK):**

```json
[
  {"id": 1, "name": "Alice", "age": 30, "role": "admin"},
  {"id": 2, "name": "Bob", "age": 25, "role": "user"},
  {"id": 3, "name": "Charlie", "age": 35, "role": "admin"}
]
```

**Example — filter by equality:**

```bash
curl "http://localhost:8080/api/users?eq.role=admin"
```

**Response:**

```json
[
  {"id": 1, "name": "Alice", "age": 30, "role": "admin"},
  {"id": 3, "name": "Charlie", "age": 35, "role": "admin"}
]
```

**Example — multiple filters (AND semantics):**

```bash
curl "http://localhost:8080/api/users?eq.role=admin&eq.age=30"
```

**Response:**

```json
[
  {"id": 1, "name": "Alice", "age": 30, "role": "admin"}
]
```

---

### Error Responses

All errors are returned as JSON with a 500 status code:

```json
{
  "error": "bucket (nonexistent) not found"
}
```

---

## Query Parameter Syntax

### Equality Filters

Query parameters follow the format:

```
?<filterType>.<fieldName>=<value>
```

| Component     | Description                                              | Example     |
|---------------|----------------------------------------------------------|-------------|
| `filterType`  | The comparison operator to apply                         | `eq`        |
| `fieldName`   | The document field to compare against                    | `name`      |
| `value`       | The value to compare with (automatically type-coerced)   | `Alice`     |

**Currently supported filter types:**

| Filter Type | Operator  | Description        | Example                  |
|-------------|-----------|--------------------|--------------------------|
| `eq`        | `==`      | Equality match     | `?eq.name=Alice`         |

Multiple query parameters are combined with **AND** semantics — a document must match all filters to be included.

### Type Coercion

Since HTTP query parameters are always strings, values are automatically coerced into their most specific Go type before comparison:

| Priority | Input          | Detected Type | Go Value           |
|----------|----------------|---------------|--------------------|
| 1        | `"true"`       | `bool`        | `true`             |
| 2        | `"false"`      | `bool`        | `false`            |
| 3        | `"42"`         | `int`         | `42`               |
| 4        | `"3.14"`       | `float64`     | `3.14`             |
| 5        | `"hello"`      | `string`      | `"hello"`          |

Integer detection is checked before float to prevent `"42"` from being parsed as `float64(42.0)`.

---

## Go SDK Usage

Applications written in Go can embed the database directly, bypassing the HTTP layer entirely for lower latency and tighter integration.

### Initializing the Database

```go
import "github.com/YpatiosCh/Database/db"

// Open with defaults (JSON encoding, file: "database.db")
database, err := db.New()
if err != nil {
    log.Fatal(err)
}
```

### Functional Options

Override defaults at construction time:

```go
database, err := db.New(
    db.WithDBName("myapp"),
    db.WithEncoder(myCustomEncoder{}),
    db.WithDecoder(myCustomDecoder{}),
)
```

| Option            | Description                                  | Default        |
|-------------------|----------------------------------------------|----------------|
| `WithDBName`      | Sets the database filename (without extension)| `"database"`  |
| `WithEncoder`     | Plugs in a custom document serializer         | `JSONEncoder`  |
| `WithDecoder`     | Plugs in a custom document deserializer       | `JSONDecoder`  |

### Insert

```go
id, err := database.Coll("users").Insert(db.M{
    "name": "Alice",
    "age":  30,
    "role": "admin",
})
// id = 1 (auto-generated, monotonically increasing)
```

### Find

```go
// Find all documents in a collection
results, err := database.Coll("users").Find()

// Find with equality filter
admins, err := database.Coll("users").Eq(db.M{"role": "admin"}).Find()
```

### Update

Updates only the fields that already exist in matching documents (patch semantics — new fields are not added):

```go
updated, err := database.Coll("users").
    Eq(db.M{"name": "Alice"}).
    Update(db.M{"age": 31})
// Alice's age is now 31; all other fields remain unchanged
```

### Delete

```go
err := database.Coll("users").
    Eq(db.M{"role": "guest"}).
    Delete()
// All documents where role == "guest" are removed
```

### Select (Field Projection)

Return only specific fields from each document:

```go
results, err := database.Coll("users").
    Select("name", "email").
    Find()
// Each result contains only "name" and "email" fields
```

### Chaining Filters

All query builder methods return the `Filter` itself, enabling fluent chaining:

```go
results, err := database.Coll("users").
    Eq(db.M{"role": "admin"}).
    Select("name", "age").
    Limit(10).
    Find()
```

---

## Custom Encoding

The database serialization is abstracted behind two interfaces:

```go
type DataEncoder interface {
    Encode(M) ([]byte, error)
}

type DataDecoder interface {
    Decode([]byte, any) error
}
```

To use a custom format (e.g., MessagePack):

```go
type MsgpackEncoder struct{}

func (MsgpackEncoder) Encode(data db.M) ([]byte, error) {
    return msgpack.Marshal(data)
}

type MsgpackDecoder struct{}

func (MsgpackDecoder) Decode(b []byte, v any) error {
    return msgpack.Unmarshal(b, v)
}

// Use it:
database, err := db.New(
    db.WithEncoder(MsgpackEncoder{}),
    db.WithDecoder(MsgpackDecoder{}),
)
```

---

## Project Structure

```
database-golang/
├── cmd/
│   └── main.go            # Entry point — wires DB, API, and HTTP server
├── api/
│   ├── api.go             # HTTP handlers (POST insert, GET query)
│   └── filter_map.go      # Query parameter parsing and type coercion
├── db/
│   ├── db.go              # Core DB struct, constructor, collection access
│   ├── options.go         # Functional options (WithDBName, WithEncoder, etc.)
│   ├── encoding.go        # DataEncoder/DataDecoder interfaces + JSON impls
│   ├── filter.go          # Query builder (Filter) with CRUD operations
│   └── utils.go           # Binary encoding helpers (uint64 ↔ []byte)
├── Makefile               # Build and run targets
├── go.mod                 # Module definition and dependencies
└── go.sum                 # Dependency checksums
```

---

## Configuration

| Setting         | Default        | How to Change                              |
|-----------------|----------------|--------------------------------------------|
| Server port     | `8080`         | Modify `e.Start(":8080")` in `cmd/main.go` |
| Database file   | `database.db`  | Pass `db.WithDBName("custom")` to `db.New`  |
| Encoder         | JSON           | Pass `db.WithEncoder(...)` to `db.New`       |
| Decoder         | JSON           | Pass `db.WithDecoder(...)` to `db.New`       |
| File permissions| `0666`         | Modify `bbolt.Open` call in `db/db.go`       |

---

## Dependencies

| Dependency                                                  | Purpose                                  |
|-------------------------------------------------------------|------------------------------------------|
| [bbolt](https://github.com/etcd-io/bbolt) `v1.4.3`        | Embedded B+ tree key-value storage engine |
| [echo](https://github.com/labstack/echo) `v5.1.0`         | HTTP router and server framework          |
| [google/uuid](https://github.com/google/uuid) `v1.6.0`    | UUID generation (indirect dependency)     |

---

## License

This project is provided as-is for educational and development purposes.
