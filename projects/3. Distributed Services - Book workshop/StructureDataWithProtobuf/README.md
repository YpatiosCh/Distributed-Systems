# Chapter 2: Structure Data with Protocol Buffers

## Overview

This chapter introduces Protocol Buffers (protobuf), Google's language-neutral data serialization format. You'll learn why binary protocols are superior to JSON for distributed systems and how to define, generate, and use protobuf messages in Go.

## What You'll Learn

- Defining data schemas with Protocol Buffers
- Compiling `.proto` files to Go code
- Using generated code for serialization/deserialization
- Understanding protobuf's efficiency advantages
- Schema evolution and backward compatibility

## Why Protocol Buffers?

### Problems with JSON
- **Verbose** - Wastes bandwidth and storage
- **Slow** - Text parsing is CPU-intensive
- **No Schema** - No compile-time validation
- **Type Unsafe** - Easy to make mistakes
- **Ambiguous** - Number types, date formats vary

### Protobuf Advantages
- **Compact** - 3-10x smaller than JSON
- **Fast** - 20-100x faster to parse
- **Strongly Typed** - Compile-time safety
- **Schema-First** - Clear contracts between services
- **Backward Compatible** - Easy to evolve
- **Multi-Language** - Code generation for many languages

## Project Structure

```
StructureDataWithProtobuf/
├── api/
│   └── v1/
│       ├── log.proto         # Protocol Buffer schema definition
│       └── log.pb.go         # Generated Go code (do not edit)
├── go.mod
└── go.sum
```

## The Protocol Buffer Schema

**File: `api/v1/log.proto`**

```protobuf
syntax = "proto3";

package log.v1;

option go_package = "github.com/igor-baiborodine/api/log_v1";

message Record {
  bytes  value = 1;   // The actual data
  uint64 offset = 2;  // Position in the log
}
```

### Key Elements

- **syntax = "proto3"** - Uses Protocol Buffers version 3
- **package log.v1** - Protobuf package name (for namespacing)
- **go_package** - Determines the Go import path
- **Field Numbers** - `= 1`, `= 2` are permanent identifiers (never change!)
- **Field Types** - `bytes` for binary data, `uint64` for unsigned integers

### Field Numbering Rules

⚠️ **CRITICAL**: Field numbers are part of the binary format
- Numbers 1-15 use 1 byte (use for frequently set fields)
- Numbers 16-2047 use 2 bytes
- Never reuse deleted field numbers
- Never change field numbers (breaks compatibility)

## Code Generation

### Installing protoc compiler

**macOS:**
```bash
brew install protobuf
```

**Linux:**
```bash
apt install -y protobuf-compiler
```

**Install Go plugin:**
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### Generate Go Code

```bash
# From the StructureDataWithProtobuf directory
protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  api/v1/*.proto
```

This creates `api/v1/log.pb.go` with:
- Struct definitions for your messages
- Marshal/Unmarshal methods
- Getters for all fields
- Type-safe API

## Using Generated Code

### Creating a Record

```go
import api "github.com/igor-baiborodine/proglog/api/v1"

record := &api.Record{
    Value:  []byte("hello world"),
    Offset: 0,
}
```

### Serializing (Marshal)

```go
import "google.golang.org/protobuf/proto"

data, err := proto.Marshal(record)
if err != nil {
    log.Fatal(err)
}
// data is a compact byte slice ready for network/disk
```

### Deserializing (Unmarshal)

```go
record := &api.Record{}
err := proto.Unmarshal(data, record)
if err != nil {
    log.Fatal(err)
}
fmt.Println(string(record.Value))  // "hello world"
```

## Schema Evolution

Protobuf supports backward and forward compatibility:

### Adding Fields (Safe)
```protobuf
message Record {
  bytes  value = 1;
  uint64 offset = 2;
  string metadata = 3;  // New field - old code ignores it
}
```

### Removing Fields (Use with Care)
```protobuf
message Record {
  reserved 3;           // Reserve the number
  reserved "metadata";  // Reserve the name
  bytes  value = 1;
  uint64 offset = 2;
}
```

### Renaming Fields (Safe)
Field names are just metadata - you can rename them freely since the binary format uses field numbers.

### Changing Types (Dangerous)
Only certain type changes are safe:
- int32 ↔ int64
- sint32 ↔ sint64
- uint32 ↔ uint64
- bytes ↔ string

## Performance Comparison

Typical benchmarks (varies by data):

| Format | Size | Encode Speed | Decode Speed |
|--------|------|--------------|--------------|
| JSON | 100% | 1x | 1x |
| Protobuf | 30% | 20x | 50x |

For a 1KB JSON message:
- Protobuf: ~300 bytes
- JSON: ~1000 bytes
- **Savings**: 70% bandwidth, 10x faster

## Best Practices

### 1. Version Your APIs
```protobuf
package log.v1  // Version in package name
```

### 2. Use Field Numbers Wisely
```protobuf
message Record {
  bytes value = 1;      // Frequent field - low number
  uint64 offset = 2;    // Frequent field - low number
  string metadata = 20; // Rare field - higher number OK
}
```

### 3. Add Comments
```protobuf
// Record represents a single entry in the commit log.
message Record {
  // The actual data stored in the record.
  bytes value = 1;
  
  // The record's position in the log (0-indexed).
  uint64 offset = 2;
}
```

### 4. Organize Proto Files
```
api/
└── v1/
    ├── log.proto      # Domain: log operations
    ├── error.proto    # Domain: error codes
    └── health.proto   # Domain: health checks
```

## Common Pitfalls

❌ **Changing field numbers** - Breaks binary compatibility  
❌ **Reusing field numbers** - Causes data corruption  
❌ **Required fields** - Removed in proto3, don't rely on them  
❌ **Large messages** - Consider streaming for data > 1MB  
❌ **Deeply nested** - Keep hierarchy shallow for performance  

## Dependencies

```
google.golang.org/protobuf v1.28.1  # Protobuf runtime for Go
```

## Testing Protobuf Code

```go
func TestRecordSerialization(t *testing.T) {
    original := &api.Record{
        Value:  []byte("test data"),
        Offset: 42,
    }
    
    // Marshal
    data, err := proto.Marshal(original)
    require.NoError(t, err)
    
    // Unmarshal
    decoded := &api.Record{}
    err = proto.Unmarshal(data, decoded)
    require.NoError(t, err)
    
    // Verify
    assert.Equal(t, original.Value, decoded.Value)
    assert.Equal(t, original.Offset, decoded.Offset)
}
```

## Key Takeaways

✅ Protocol Buffers are faster and smaller than JSON  
✅ Schema-first design catches errors at compile time  
✅ Field numbers are permanent - plan carefully  
✅ Code generation reduces boilerplate and errors  
✅ Backward compatibility enables gradual rollouts  

## Next Steps

Move to **Chapter 3: Write a Log Package** to learn how to:
- Implement persistent storage with memory-mapped files
- Build efficient indexes for fast lookups
- Manage log segments and rotation
- Use protobuf for disk serialization

## Additional Resources

- [Protocol Buffers Documentation](https://developers.google.com/protocol-buffers)
- [Protocol Buffers Language Guide](https://developers.google.com/protocol-buffers/docs/proto3)
- [Go Protobuf Tutorial](https://developers.google.com/protocol-buffers/docs/gotutorial)
- [Encoding Guide](https://developers.google.com/protocol-buffers/docs/encoding)

---

**Ready to build real storage?** The next chapter implements a production-grade log with persistent storage! 💾