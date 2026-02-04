# Distributed Systems Learning Path 🚀

> A hands-on journey through distributed systems fundamentals, from basic concepts to production-ready implementations in Go.

---

## 🎯 Purpose

This repository documents a practical, code-first approach to learning distributed systems. Each project builds upon the previous one, progressively introducing core concepts, patterns, and challenges encountered in real-world distributed applications.

**Who is this for?**
- Backend engineers transitioning to distributed systems
- Students learning distributed computing
- Anyone curious about how systems like Kafka, etcd, or Kubernetes work under the hood

---

## 🗺️ Learning Path Overview

```
Foundation → Basic Distribution → Replication → Consensus → Production
    │              │                  │            │            │
Project 1      Project 2          Project 3   Projects 3-10   Project 10
Heartbeat      KV Store         Full Log      (Book Based)  Kubernetes
                                  System
```

---

## 📚 Projects

### 1️⃣ Distributed Heartbeat System
**Path:** `projects/1. Distributed-Heartbeat/`

**What You'll Learn:**
- Node-to-node communication in a cluster
- Liveness detection and failure detection
- Go concurrency patterns (goroutines, contexts, channels)
- Graceful shutdown and signal handling
- Production logging with Zap
- Metrics collection with Prometheus

**Key Concepts:**
- Heartbeat protocols
- Fault detection
- Observability fundamentals
- Process coordination

**Technologies:**
- Go 1.23+
- Uber Zap (structured logging)
- Prometheus (metrics)

**Run Example:**
```bash
# Terminal 1
go run ./cmd/node --port=8001 --peers=http://localhost:8002,http://localhost:8003

# Terminal 2
go run ./cmd/node --port=8002 --peers=http://localhost:8001,http://localhost:8003

# Terminal 3
go run ./cmd/node --port=8003 --peers=http://localhost:8001,http://localhost:8002
```

**What Makes This Special:**
- First taste of distributed computing
- Real-time failure detection
- Production-grade logging and metrics
- Clean, modular Go architecture

---

### 2️⃣ Distributed Key-Value Store
**Path:** `projects/2. Distributed-KV-Store/`

**What You'll Learn:**
- Data replication across nodes
- Consistency through hash-based synchronization
- Peer-to-peer communication patterns
- State reconciliation after network partitions
- HTTP-based distributed APIs

**Key Concepts:**
- Replication strategies
- Eventual consistency
- Hash-based state comparison
- Peer synchronization
- Store reconciliation

**Technologies:**
- Go 1.22+
- HTTP/JSON API
- SHA-256 hashing

**Architecture:**
```
Node A                Node B                Node C
  |                     |                     |
  |-- Store KV -------->|                     |
  |                     |-- Replicate ------->|
  |<-- Ping ------------|<-- Ping ------------|
  |-- Compare Hash ---->|                     |
  |                     |-- Sync Store ------>|
```

**Run Example:**
```bash
# Start 3 nodes on different ports
go run cmd/srv/main.go --port=8001 --peers=http://localhost:8002,http://localhost:8003
go run cmd/srv/main.go --port=8002 --peers=http://localhost:8001,http://localhost:8003
go run cmd/srv/main.go --port=8003 --peers=http://localhost:8001,http://localhost:8002

# Store a key-value pair (replicates to all nodes)
curl -X POST http://localhost:8001/store \
  -H "Content-Type: application/json" \
  -d '{"key": "username", "value": "alice"}'

# Retrieve from any node
curl -X GET http://localhost:8002/store/key \
  -H "Content-Type: application/json" \
  -d '{"key": "username"}'
```

**What Makes This Special:**
- Implements eventual consistency
- Automatic peer recovery
- Hash-based state comparison (efficient!)
- Real-world replication patterns

---

### 3️⃣ Distributed Services with Go (Book Workshop)
**Path:** `projects/3. Distributed Services - Book workshop/`

**Based on:** *"Distributed Services with Go"* by Travis Jeffery (Pragmatic Bookshelf, 2021)

This comprehensive workshop builds a production-ready distributed commit log service (similar to Apache Kafka) through 10 progressive chapters.

#### Chapter Breakdown

| Chapter | Topic | Key Learnings |
|---------|-------|---------------|
| **1. Let's Go** | HTTP Server Basics | REST APIs, JSON serialization, Gorilla Mux |
| **2. Structure Data** | Protocol Buffers | Binary serialization, schema evolution, efficiency |
| **3. Write a Log** | Persistent Storage | Memory-mapped files, segments, indexes, append-only logs |
| **4. Serve Requests** | gRPC Service | Streaming RPCs, HTTP/2, bidirectional communication |
| **5. Secure Services** | Security | TLS encryption, mutual TLS authentication, ACL authorization |
| **6. Observe Systems** | Observability | Structured logging (Zap), metrics (Prometheus), distributed tracing (Jaeger) |
| **7. Server-Side Discovery** | Load Balancing | DNS resolution, custom gRPC resolvers, health checking |
| **8. Client-Side Discovery** | Gossip Protocol | Serf membership, SWIM protocol, failure detection |
| **9. Consensus** | Raft Algorithm | Leader election, log replication, strong consistency |
| **10. Deploy** | Cloud Native | Kubernetes, StatefulSets, Helm charts, production deployment |

**Complete Tech Stack:**
```
┌─────────────────────────────────────────┐
│         Application Layer                │
│  ┌────────┐  ┌────────┐  ┌────────┐    │
│  │ gRPC   │  │ HTTP/2 │  │ TLS    │    │
│  └────────┘  └────────┘  └────────┘    │
├─────────────────────────────────────────┤
│       Distribution Layer                 │
│  ┌────────┐  ┌────────┐  ┌────────┐    │
│  │ Raft   │  │ Serf   │  │ DNS    │    │
│  └────────┘  └────────┘  └────────┘    │
├─────────────────────────────────────────┤
│         Storage Layer                    │
│  ┌────────┐  ┌────────┐  ┌────────┐    │
│  │ Mmap   │  │ Segment│  │ Index  │    │
│  └────────┘  └────────┘  └────────┘    │
├─────────────────────────────────────────┤
│      Observability Layer                 │
│  ┌────────┐  ┌────────┐  ┌────────┐    │
│  │ Zap    │  │Promeths│  │ Jaeger │    │
│  └────────┘  └────────┘  └────────┘    │
└─────────────────────────────────────────┘
```

**What You'll Build:**
A complete distributed log service with:
- High-performance append-only log
- Strong consistency via Raft
- Automatic failover and replication
- TLS security with client certificates
- Production-grade observability
- Kubernetes-ready deployment

**Each chapter includes:**
- Detailed README with explanations
- Working code examples
- Comprehensive tests
- Best practices and pitfalls
- Performance considerations
- Links to additional resources

---

## 🎓 Concepts Mastered

### Foundation (Projects 1-2)
- ✅ Node communication
- ✅ Heartbeat protocols
- ✅ Failure detection
- ✅ Data replication
- ✅ State synchronization
- ✅ HTTP-based APIs

### Intermediate (Project 3, Chapters 1-6)
- ✅ Protocol Buffers
- ✅ Memory-mapped files
- ✅ gRPC and streaming
- ✅ TLS encryption & mTLS
- ✅ ACL authorization
- ✅ Structured logging
- ✅ Metrics & tracing

### Advanced (Project 3, Chapters 7-10)
- ✅ Service discovery (client & server-side)
- ✅ Gossip protocols (SWIM)
- ✅ Consensus algorithms (Raft)
- ✅ Leader election
- ✅ Log replication
- ✅ Strong consistency
- ✅ Kubernetes deployment

---

## 🛠️ Technologies Used

### Languages & Frameworks
- **Go 1.22+** - Primary language
- **Protocol Buffers** - Efficient serialization
- **gRPC** - RPC framework

### Distributed Systems
- **Serf** - Gossip-based membership (Hashicorp)
- **Raft** - Consensus algorithm (Hashicorp library)
- **Casbin** - ACL authorization

### Observability
- **Uber Zap** - Structured logging
- **Prometheus** - Metrics collection
- **Jaeger** - Distributed tracing
- **OpenTelemetry** - Telemetry framework

### Infrastructure
- **Docker** - Containerization
- **Kubernetes** - Container orchestration
- **Helm** - Kubernetes package manager

---

## 📊 Complexity Progression

```
Simple → Moderate → Complex → Production-Ready
   │        │          │            │
   │        │          │            └─ Chapter 10: K8s Deployment
   │        │          └────────────── Chapter 9: Raft Consensus
   │        └───────────────────────── Project 2: KV Store
   └────────────────────────────────── Project 1: Heartbeat
```

**Lines of Code:**
- Project 1 (Heartbeat): ~500 lines
- Project 2 (KV Store): ~800 lines  
- Project 3 (Full System): ~5,000+ lines

---

## 🚀 Getting Started

### Prerequisites
- Go 1.22 or later
- Basic understanding of:
  - Go programming
  - HTTP protocols
  - Concurrent programming
- Docker (for Project 3, Chapter 10)
- Kubernetes (for deployment chapter)

### Quick Start

1. **Clone the repository**
```bash
git clone <repository-url>
cd distributed-systems-learning
```

2. **Start with Project 1**
```bash
cd projects/1.\ Distributed-Heartbeat
go mod download
# Follow the README.md
```

3. **Progress sequentially**
- Complete each project before moving to the next
- Read the chapter READMEs thoroughly
- Experiment with the code
- Break things and fix them!

---

## 🎯 Learning Recommendations

### For Beginners
1. Start with **Project 1** (Heartbeat)
   - Understand basic node communication
   - Get comfortable with Go concurrency
2. Move to **Project 2** (KV Store)
   - Learn replication basics
   - Understand eventual consistency
3. Begin **Project 3, Chapters 1-4**
   - Focus on fundamentals first
   - Take time to understand each concept

### For Intermediate Developers
- Can skip Projects 1-2 if comfortable with basics
- Focus on **Project 3, Chapters 5-9**
- Pay special attention to:
  - Security patterns (Chapter 5)
  - Consensus algorithms (Chapter 9)
  - Service discovery (Chapters 7-8)

### For Advanced Engineers
- Use as a reference implementation
- Focus on **Project 3, Chapters 9-10**
- Study the Raft implementation deeply
- Experiment with production deployment

---

## 🧪 Testing Philosophy

All projects include:
- **Unit tests** - Individual component testing
- **Integration tests** - Multi-component scenarios
- **End-to-end tests** - Full system validation

Run tests in any project:
```bash
go test -v ./...
```

---

## 📖 Additional Resources

### Books
- **"Distributed Services with Go"** by Travis Jeffery
- **"Designing Data-Intensive Applications"** by Martin Kleppmann
- **"Database Internals"** by Alex Petrov

### Papers
- [Raft Consensus Algorithm](https://raft.github.io/raft.pdf)
- [SWIM: Scalable Membership Protocol](https://www.cs.cornell.edu/projects/Quicksilver/public_pdfs/SWIM.pdf)
- [The Log: What every software engineer should know](https://engineering.linkedin.com/distributed-systems/log-what-every-software-engineer-should-know-about-real-time-datas-unifying)

### Online Resources
- [Raft Visualization](http://thesecretlivesofdata.com/raft/)
- [Prometheus Documentation](https://prometheus.io/docs/)
- [gRPC Documentation](https://grpc.io/docs/)
- [Kubernetes Documentation](https://kubernetes.io/docs/)

---

## 🗺️ What's Next?

After completing this learning path, you'll be ready to:

1. **Build Production Systems**
   - Design distributed microservices
   - Implement fault-tolerant architectures
   - Deploy to cloud platforms

2. **Explore Advanced Topics**
   - Conflict-free Replicated Data Types (CRDTs)
   - Vector clocks and causal consistency
   - Distributed transactions (2PC, 3PC, Saga)
   - Distributed tracing and debugging

3. **Contribute to Open Source**
   - etcd, Consul, Kafka
   - Kubernetes controllers
   - Cloud-native projects

4. **Dive Deeper**
   - Study other consensus algorithms (Paxos, ZAB)
   - Explore different storage engines
   - Build custom distributed systems

---

## 🤝 Contributing

This repository is a learning resource. Contributions welcome:
- Fix bugs or typos
- Improve documentation
- Add tests
- Share your learnings
- Suggest new projects

---

## 📝 Progress Tracker

Use this to track your learning journey:

- [ ] **Project 1: Distributed Heartbeat**
  - [ ] Run 3-node cluster
  - [ ] Understand failure detection
  - [ ] View Prometheus metrics
  - [ ] Experiment with node failures

- [ ] **Project 2: Distributed KV Store**
  - [ ] Deploy multi-node cluster
  - [ ] Test replication
  - [ ] Trigger peer recovery
  - [ ] Understand hash-based sync

- [ ] **Project 3: Distributed Services (Book)**
  - [ ] Chapter 1: Let's Go
  - [ ] Chapter 2: Structure Data with Protobuf
  - [ ] Chapter 3: Write a Log Package
  - [ ] Chapter 4: Serve Requests with gRPC
  - [ ] Chapter 5: Secure Your Services
  - [ ] Chapter 6: Observe Your Systems
  - [ ] Chapter 7: Server-Side Discovery
  - [ ] Chapter 8: Client-Side Discovery
  - [ ] Chapter 9: Coordinate with Consensus
  - [ ] Chapter 10: Deploy to Cloud

---

## 👨‍💻 Author

Created by **[YpatiosCh](https://github.com/YpatiosCh)** as part of a personal journey to master distributed systems engineering.

This repository represents months of learning, experimentation, and hands-on implementation of distributed systems concepts.

---

## 🌟 Acknowledgments

- **Travis Jeffery** - For the excellent "Distributed Services with Go" book
- **HashiCorp** - For open-sourcing Raft and Serf implementations
- **The Pragmatic Programmers** - For publishing quality technical books
- The **Go community** - For excellent libraries and tooling
- The **distributed systems community** - For sharing knowledge and research

---

## 📜 License

This repository is licensed under the MIT License. See individual project directories for specific licensing information.

---

## 💡 Final Thoughts

> "The best way to learn distributed systems is to build them."

This repository is proof that complex systems can be learned incrementally. Each project builds confidence and understanding. Don't rush - take time to understand each concept deeply.

Distributed systems are challenging, but incredibly rewarding. They power the modern internet, from Netflix to Twitter to your favorite apps.

**Welcome to the journey. Let's build something distributed! 🚀**

---

## 📫 Questions or Feedback?

Feel free to:
- Open an issue for questions
- Submit a PR for improvements
- Share your learning journey
- Connect on GitHub

Happy learning! 🎓