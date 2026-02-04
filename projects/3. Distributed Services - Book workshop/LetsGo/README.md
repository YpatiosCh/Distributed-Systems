# Chapter 1: Let's Go - Building Your First Service

## Overview

This chapter introduces the foundational concepts of building a distributed service by creating a simple HTTP server that handles log records. You'll learn the basics of REST APIs, JSON serialization, and Go's HTTP server capabilities.

## What You'll Learn

- Setting up a Go project with modules
- Creating HTTP handlers with the Gorilla Mux router
- Implementing a simple in-memory log store
- Writing and reading log records via REST API
- Basic error handling and HTTP status codes

## Key Concepts

### The Log Abstraction
A log is an append-only sequence of records ordered by time. This simple abstraction is fundamental to many distributed systems including:
- Apache Kafka
- Amazon Kinesis
- NATS Streaming
- Traditional databases (write-ahead logs)

### HTTP Server Pattern
The chapter demonstrates building a server with:
- **POST /log** - Append a record to the log
- **GET /log** - Read a record from the log by offset

## Project Structure

```
LetsGo/
├── cmd/
│   └── server/
│       └── main.go           # Entry point - starts the HTTP server
├── internal/
│   └── server/
│       ├── http.go           # HTTP server and handler implementation
│       └── log.go            # In-memory log data structure
├── go.mod                    # Go module definition
└── go.sum                    # Dependency checksums
```

## Implementation Details

### The Log Data Structure (`internal/server/log.go`)

The log is implemented as a slice of records:

```go
type Log struct {
    mu      sync.Mutex
    records []Record
}

type Record struct {
    Value  []byte `json:"value"`
    Offset uint64 `json:"offset"`
}
```

Key operations:
- `Append(record Record)` - Adds a record and returns its offset
- `Read(offset uint64)` - Returns the record at the given offset

### HTTP Handlers (`internal/server/http.go`)

Two main handlers:
- `handleProduce` - Handles POST requests to append records
- `handleConsume` - Handles GET requests to read records

The server uses Gorilla Mux for routing, which provides:
- Path variables
- HTTP method routing
- Middleware support

## Running the Server

```bash
# From the LetsGo directory
go run cmd/server/main.go
```

The server starts on `localhost:8080`

## Testing the API

### Append a record:
```bash
curl -X POST http://localhost:8080/log \
  -H "Content-Type: application/json" \
  -d '{"value":"aGVsbG8gd29ybGQ="}'  # "hello world" base64 encoded
```

Response:
```json
{"offset":0}
```

### Read a record:
```bash
curl http://localhost:8080/log?offset=0
```

Response:
```json
{"value":"aGVsbG8gd29ybGQ=","offset":0}
```

## Limitations & Lessons

This initial implementation is intentionally simple and has several limitations:

1. **No Persistence** - Data is lost when the server restarts
2. **No Concurrency Control** - Uses a simple mutex (not scalable)
3. **JSON Overhead** - JSON is verbose and slow for high-throughput systems
4. **No Error Recovery** - Crashes lose all data
5. **Single Server** - No replication or fault tolerance

These limitations will be addressed in subsequent chapters:
- Chapter 3 adds persistent storage
- Chapter 2 improves serialization with Protocol Buffers
- Later chapters add replication, consensus, and distribution

## Key Takeaways

✅ A log is a simple but powerful abstraction  
✅ HTTP/JSON provides an accessible API for prototyping  
✅ Go's standard library makes building servers straightforward  
✅ Even simple systems need proper error handling  
✅ Production systems require much more than basic CRUD operations  

## Next Steps

Move to **Chapter 2: Structure Data with Protocol Buffers** to learn how to:
- Define schemas with Protocol Buffers
- Generate efficient serialization code
- Reduce network overhead
- Ensure backward compatibility

## Dependencies

```
github.com/gorilla/mux v1.7.3  # HTTP router with more features than net/http
```

## Common Issues

**Port already in use?**
```bash
# Find and kill the process using port 8080
lsof -i :8080
kill -9 <PID>
```

**Import errors?**
```bash
go mod tidy
go mod download
```

## Additional Resources

- [Gorilla Mux Documentation](https://github.com/gorilla/mux)
- [Go HTTP Server Tutorial](https://golang.org/doc/tutorial/web-service-gin)
- [The Log: What every software engineer should know](https://engineering.linkedin.com/distributed-systems/log-what-every-software-engineer-should-know-about-real-time-datas-unifying)

---

**Ready to continue?** Head to the next chapter to make your service more efficient with Protocol Buffers! 📦