# Chapter 4: Serve Requests with gRPC

## Overview

This chapter wraps your persistent log in a gRPC service, enabling network communication between clients and servers. You'll learn about Protocol Buffers service definitions, bidirectional streaming, and building production-grade RPC servers.

## What You'll Learn

- Defining gRPC services in Protocol Buffers
- Implementing unary and streaming RPC methods
- Building gRPC servers and clients in Go
- Handling bidirectional streams
- Multiplexing connections with `cmux`
- Testing gRPC services

## Why gRPC?

### Problems with REST/HTTP
- Verbose (JSON overhead)
- No type safety
- No streaming support
- Inefficient connection handling
- Manual serialization

### gRPC Advantages
- **Fast**: Binary protocol with HTTP/2
- **Streaming**: Bidirectional, server, and client streaming
- **Type-Safe**: Generated code from protobuf
- **Multi-Language**: Client/server in different languages
- **Efficient**: Connection multiplexing, header compression
- **Modern**: Built-in load balancing, auth, monitoring

## Project Structure

```
ServeRequests/
├── api/
│   └── v1/
│       ├── log.proto          # Service definition
│       ├── log.pb.go          # Generated message code
│       └── log_grpc.pb.go     # Generated service code
├── internal/
│   ├── log/
│   │   └── ...                # Log implementation (from Ch. 3)
│   └── server/
│       ├── server.go          # gRPC server implementation
│       ├── server_test.go     # Integration tests
│       └── log.go             # CommitLog interface
├── cmd/
│   └── server/
│       └── main.go            # Server entry point
├── go.mod
└── go.sum
```

## Protocol Buffer Service Definition

**File: `api/v1/log.proto`**

```protobuf
syntax = "proto3";

package log.v1;

service Log {
  // Unary RPC: single request, single response
  rpc Produce(ProduceRequest) returns (ProduceResponse) {}
  
  // Unary RPC: single request, single response
  rpc Consume(ConsumeRequest) returns (ConsumeResponse) {}
  
  // Server streaming: single request, stream of responses
  rpc ConsumeStream(ConsumeRequest) returns (stream ConsumeResponse) {}
  
  // Bidirectional streaming
  rpc ProduceStream(stream ProduceRequest) returns (stream ProduceResponse) {}
}

message ProduceRequest {
  Record record = 1;
}

message ProduceResponse {
  uint64 offset = 1;
}

message ConsumeRequest {
  uint64 offset = 1;
}

message ConsumeResponse {
  Record record = 2;
}

message Record {
  bytes  value = 1;
  uint64 offset = 2;
}
```

### Service Methods

1. **Produce** (Unary)
   - Client sends one record
   - Server appends and returns offset
   - Use case: Write a single record

2. **Consume** (Unary)
   - Client requests offset
   - Server returns one record
   - Use case: Fetch a specific record

3. **ConsumeStream** (Server Streaming)
   - Client sends starting offset
   - Server streams all subsequent records
   - Use case: Tail the log, consume multiple records

4. **ProduceStream** (Bidirectional Streaming)
   - Client streams records
   - Server streams back offsets
   - Use case: Batch ingestion with feedback

## Code Generation

```bash
# Install gRPC plugin
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate code
protoc \
  --go_out=. \
  --go_opt=paths=source_relative \
  --go-grpc_out=. \
  --go-grpc_opt=paths=source_relative \
  api/v1/*.proto
```

This generates:
- `log.pb.go` - Message types
- `log_grpc.pb.go` - Service interfaces and client

## Server Implementation

### CommitLog Interface

```go
type CommitLog interface {
    Append(*api.Record) (uint64, error)
    Read(uint64) (*api.Record, error)
}
```

This abstraction allows testing without a real log.

### gRPC Server Structure

```go
type grpcServer struct {
    api.UnimplementedLogServer
    *Config
}

type Config struct {
    CommitLog CommitLog
}

func NewGRPCServer(config *Config) (*grpc.Server, error) {
    gsrv := grpc.NewServer()
    srv := &grpcServer{
        Config: config,
    }
    api.RegisterLogServer(gsrv, srv)
    return gsrv, nil
}
```

### Implementing Unary RPCs

**Produce:**
```go
func (s *grpcServer) Produce(
    ctx context.Context,
    req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
    offset, err := s.CommitLog.Append(req.Record)
    if err != nil {
        return nil, err
    }
    return &api.ProduceResponse{Offset: offset}, nil
}
```

**Consume:**
```go
func (s *grpcServer) Consume(
    ctx context.Context,
    req *api.ConsumeRequest,
) (*api.ConsumeResponse, error) {
    record, err := s.CommitLog.Read(req.Offset)
    if err != nil {
        return nil, err
    }
    return &api.ConsumeResponse{Record: record}, nil
}
```

### Implementing Server Streaming

**ConsumeStream:**
```go
func (s *grpcServer) ConsumeStream(
    req *api.ConsumeRequest,
    stream api.Log_ConsumeStreamServer,
) error {
    for {
        select {
        case <-stream.Context().Done():
            return nil
        default:
            res, err := s.Consume(stream.Context(), req)
            switch err.(type) {
            case nil:
            case api.ErrOffsetOutOfRange:
                continue
            default:
                return err
            }
            if err = stream.Send(res); err != nil {
                return err
            }
            req.Offset++
        }
    }
}
```

**Key Points:**
- Loops continuously sending records
- Respects context cancellation
- Handles end-of-log gracefully
- Auto-increments offset

### Implementing Bidirectional Streaming

**ProduceStream:**
```go
func (s *grpcServer) ProduceStream(
    stream api.Log_ProduceStreamServer,
) error {
    for {
        req, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        res, err := s.Produce(stream.Context(), req)
        if err != nil {
            return err
        }
        if err = stream.Send(res); err != nil {
            return err
        }
    }
}
```

**Key Points:**
- Receives from client stream
- Produces record
- Sends offset back
- Continues until client closes

## Running the Server

### Basic Setup

```go
func main() {
    log := ... // Create your log from Chapter 3
    
    config := &Config{
        CommitLog: log,
    }
    
    server, err := NewGRPCServer(config)
    if err != nil {
        log.Fatal(err)
    }
    
    l, err := net.Listen("tcp", ":8080")
    if err != nil {
        log.Fatal(err)
    }
    
    if err := server.Serve(l); err != nil {
        log.Fatal(err)
    }
}
```

## Client Usage

### Creating a Client

```go
conn, err := grpc.Dial(
    "localhost:8080",
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := api.NewLogClient(conn)
```

### Unary RPC Calls

```go
// Produce
produceRes, err := client.Produce(
    context.Background(),
    &api.ProduceRequest{
        Record: &api.Record{
            Value: []byte("hello world"),
        },
    },
)

// Consume
consumeRes, err := client.Consume(
    context.Background(),
    &api.ConsumeRequest{
        Offset: produceRes.Offset,
    },
)
```

### Server Streaming

```go
stream, err := client.ConsumeStream(
    context.Background(),
    &api.ConsumeRequest{Offset: 0},
)
if err != nil {
    log.Fatal(err)
}

for {
    res, err := stream.Recv()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(res.Record.Value))
}
```

### Bidirectional Streaming

```go
stream, err := client.ProduceStream(context.Background())
if err != nil {
    log.Fatal(err)
}

// Send records
for i := 0; i < 10; i++ {
    err := stream.Send(&api.ProduceRequest{
        Record: &api.Record{
            Value: []byte(fmt.Sprintf("record %d", i)),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    
    // Receive offset
    res, err := stream.Recv()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Offset: %d\n", res.Offset)
}
```

## Testing gRPC Services

### Test Setup

```go
func setupTest(t *testing.T, fn func(*Config)) (
    client api.LogClient,
    teardown func(),
) {
    t.Helper()
    
    l, err := net.Listen("tcp", "127.0.0.1:0")
    require.NoError(t, err)
    
    config := &Config{
        CommitLog: &mockLog{},
    }
    if fn != nil {
        fn(config)
    }
    
    server, err := NewGRPCServer(config)
    require.NoError(t, err)
    
    go func() {
        server.Serve(l)
    }()
    
    conn, err := grpc.Dial(
        l.Addr().String(),
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    require.NoError(t, err)
    
    client = api.NewLogClient(conn)
    
    return client, func() {
        server.Stop()
        conn.Close()
        l.Close()
    }
}
```

### Example Test

```go
func TestProduceConsume(t *testing.T) {
    client, teardown := setupTest(t, nil)
    defer teardown()
    
    // Produce
    want := &api.Record{Value: []byte("hello world")}
    produce, err := client.Produce(
        context.Background(),
        &api.ProduceRequest{Record: want},
    )
    require.NoError(t, err)
    
    // Consume
    consume, err := client.Consume(
        context.Background(),
        &api.ConsumeRequest{Offset: produce.Offset},
    )
    require.NoError(t, err)
    require.Equal(t, want.Value, consume.Record.Value)
}
```

## Connection Multiplexing with cmux

For production, you might want multiple protocols on one port:

```go
import "github.com/soheilhy/cmux"

l, err := net.Listen("tcp", ":8080")
m := cmux.New(l)

grpcL := m.Match(cmux.HTTP2HeaderField(
    "content-type",
    "application/grpc",
))
httpL := m.Match(cmux.HTTP1Fast())

grpcServer := ... // Your gRPC server
httpServer := ... // Your HTTP server

go grpcServer.Serve(grpcL)
go httpServer.Serve(httpL)

m.Serve()
```

## Performance Considerations

### HTTP/2 Benefits
- **Multiplexing**: Multiple requests on one connection
- **Header Compression**: Reduces overhead
- **Server Push**: Preemptive sending
- **Flow Control**: Prevents overwhelming

### Streaming Benefits
- **Lower Latency**: No connection setup per message
- **Memory Efficient**: No buffering all data
- **Real-Time**: Immediate feedback
- **Backpressure**: Flow control built-in

### Best Practices
- Use connection pooling
- Set reasonable timeouts
- Implement retry logic
- Monitor connection health
- Use keepalive settings

## Error Handling

### gRPC Status Codes

```go
import (
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
)

// Return typed errors
if offset > s.log.HighestOffset() {
    return nil, status.Error(
        codes.OutOfRange,
        "offset out of range",
    )
}
```

### Common Codes
- `OK` - Success
- `Canceled` - Client canceled
- `InvalidArgument` - Bad request
- `NotFound` - Resource not found
- `AlreadyExists` - Conflict
- `PermissionDenied` - Auth error
- `ResourceExhausted` - Quota exceeded
- `FailedPrecondition` - Invalid state
- `Unavailable` - Service down
- `Internal` - Server error

## Key Takeaways

✅ gRPC provides type-safe, efficient RPC  
✅ Streaming enables real-time, memory-efficient communication  
✅ HTTP/2 multiplexing improves connection utilization  
✅ Generated code reduces boilerplate and errors  
✅ Proper error handling improves client experience  

## Next Steps

Move to **Chapter 5: Secure Your Services** to learn how to:
- Add TLS encryption
- Implement authentication with client certificates
- Add authorization with ACLs
- Secure inter-service communication

## Dependencies

```
github.com/soheilhy/cmux v0.1.5                    # Connection multiplexing
google.golang.org/grpc v1.50.1                     # gRPC framework
google.golang.org/protobuf v1.28.1                 # Protocol Buffers
```

## Additional Resources

- [gRPC Documentation](https://grpc.io/docs/)
- [gRPC Go Tutorial](https://grpc.io/docs/languages/go/basics/)
- [HTTP/2 Specification](https://http2.github.io/)
- [gRPC Best Practices](https://grpc.io/docs/guides/performance/)

---

**Ready to secure your service?** The next chapter adds authentication and authorization! 🔐