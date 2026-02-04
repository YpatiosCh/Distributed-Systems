# Distributed Services with Go - Book Workshop

A comprehensive hands-on workshop based on Travis Jeffery's book **"Distributed Services with Go: Your Guide to Reliable, Scalable, and Maintainable Systems"** (Pragmatic Bookshelf, 2021).

## About This Workshop

This repository contains a complete implementation of a distributed commit log service built from the ground up. Each directory represents a chapter in the learning journey, progressively building a production-ready distributed system with features like replication, consensus, service discovery, observability, and cloud deployment.

## What You'll Build

By the end of this workshop, you'll have constructed a fully functional distributed log service (similar to Apache Kafka) with:

- **High-performance log storage** with memory-mapped files
- **gRPC-based API** for client-server communication
- **TLS authentication and authorization** using mutual TLS and access control lists
- **Service discovery** using both client-side (with Serf) and server-side approaches
- **Consensus and replication** using the Raft algorithm
- **Observability** with structured logging, metrics, and distributed tracing
- **Cloud deployment** on Kubernetes with Helm charts

## Project Structure

Each directory represents a progressive chapter building upon the previous one:

```
.
├── LetsGo/                          # Chapter 1: Getting Started
├── StructureDataWithProtobuf/       # Chapter 2: Data Serialization
├── WriteALogPackage/                # Chapter 3: Core Log Implementation
├── ServeRequests/                   # Chapter 4: gRPC Service Layer
├── SecureServices/                  # Chapter 5: Authentication & Authorization
├── ObserveYourSystems/              # Chapter 6: Logging, Metrics & Tracing
├── ServerSideServiceDiscovery/      # Chapter 7: Server-Side Discovery
├── ClientSideServiceDiscovery/      # Chapter 8: Client-Side Discovery with Serf
├── CoordinateWithConsensus/         # Chapter 9: Raft Consensus & Replication
└── DeployToCloud/                   # Chapter 10: Kubernetes Deployment
```

## Prerequisites

- **Go 1.23.3** or later
- **Protocol Buffers compiler** (protoc)
- Basic understanding of:
  - Go programming language
  - Distributed systems concepts
  - gRPC and Protocol Buffers
  - Docker and Kubernetes (for deployment chapter)

## Learning Path

### Foundation (Chapters 1-3)
Build the core data structures and storage layer:
- Simple HTTP server with JSON API
- Protocol Buffers for efficient serialization
- Append-only log with segments, indexes, and memory-mapped files

### Distribution (Chapters 4-6)
Add production-grade features:
- gRPC service with streaming support
- Mutual TLS authentication and ACL-based authorization
- Comprehensive observability with OpenTelemetry

### Clustering (Chapters 7-9)
Make it distributed:
- Service discovery with multiple approaches
- Raft consensus for strong consistency
- Multi-server replication and leader election

### Production (Chapter 10)
Deploy to the cloud:
- Kubernetes StatefulSets and Services
- Helm charts for easy deployment
- Cloud-native architecture patterns

## Key Technologies

- **Go** - Primary programming language
- **Protocol Buffers** - Data serialization
- **gRPC** - RPC framework
- **Serf** - Cluster membership and failure detection
- **Raft** (via Hashicorp's library) - Consensus algorithm
- **OpenTelemetry** - Observability framework
- **Kubernetes** - Container orchestration
- **Helm** - Kubernetes package manager

## Architecture Highlights

### Storage Layer
- Append-only log with automatic segment rotation
- Memory-mapped files for zero-copy reads
- Efficient indexing for fast lookups

### Network Layer
- Bidirectional streaming with gRPC
- Multiplexed connections
- TLS encryption with client certificates

### Replication Layer
- Leader-based replication via Raft
- Strong consistency guarantees
- Automatic failover and recovery

### Discovery Layer
- Both client-side and server-side patterns
- Gossip-based cluster membership
- Health checking and load balancing

## Testing

Each chapter includes comprehensive tests demonstrating:
- Unit tests for individual components
- Integration tests for multi-component scenarios
- End-to-end tests for full system validation

Run tests in any chapter:
```bash
go test -v ./...
```

## Book Reference

This workshop closely follows the structure and examples from:

**"Distributed Services with Go: Your Guide to Reliable, Scalable, and Maintainable Systems"**  
by Travis Jeffery  
Pragmatic Bookshelf, 2021

[Purchase the book](https://pragprog.com/titles/tjgo/distributed-services-with-go/)

## Contributing

This is a learning workshop repository. Feel free to:
- Report issues or bugs
- Suggest improvements
- Share your own implementations
- Ask questions in discussions

## License

This workshop code is provided for educational purposes. Please refer to the book's license for the original content and examples.

## Resources

- [Official Book Website](https://pragprog.com/titles/tjgo/)
- [Go Documentation](https://golang.org/doc/)
- [gRPC Documentation](https://grpc.io/docs/)
- [Raft Paper](https://raft.github.io/raft.pdf)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)

## Acknowledgments

Special thanks to Travis Jeffery for writing an excellent guide to building distributed systems in Go, and to The Pragmatic Programmers for publishing this invaluable resource for the Go community.

---

**Happy Learning! 🚀**

Start with `LetsGo/` and build your way to a production-ready distributed system!