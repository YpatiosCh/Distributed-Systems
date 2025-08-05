# Distributed Heartbeat System in Go

> A production-style, beginner-friendly project to learn distributed systems through a real-world heartbeat monitoring system. Designed to function as both a practical tool and an educational course for aspiring backend and distributed systems engineers.

---

## 🎯 Purpose & Learning Objectives

This project is an educational journey through the foundations of distributed systems. It teaches how to:

* Build a basic multi-node cluster system
* Implement node-to-node liveness detection (heartbeats)
* Use Go concurrency with goroutines and context
* Handle graceful shutdowns and signal termination
* Log using structured, production-grade logs (Zap)
* Expose real-time metrics for observability (Prometheus)

This repository is intended to both **teach** and **demonstrate** a deployable, modular, extensible distributed application.

---

## 🧠 Distributed Systems Concepts Covered

| Concept            | Description                                                 |
| ------------------ | ----------------------------------------------------------- |
| Node               | A self-contained process in a distributed system            |
| Heartbeat          | Regular signal to show liveness                             |
| Liveness Detection | Determining if a node is alive or unresponsive              |
| Fault Tolerance    | Withstand individual node failures without affecting others |
| Observability      | Metrics and logging for real-time status visibility         |

---

## 📁 Project Structure (Modular Go Layout)

```bash
📦 distributed-heartbeat/
├── cmd/                    # Application entrypoint(s)
│   └── node/              # CLI node process
├── internal/              # Internal packages (domain logic)
│   ├── config/            # CLI flags, environment vars
│   ├── server/            # HTTP server with /ping endpoint
│   ├── monitor/           # Heartbeat sender and tracker
│   ├── logging/           # Zap logger configuration
│   └── metrics/           # Prometheus counters and handler
├── go.mod
└── README.md              # This file
```

---

## 🛠️ Setup Instructions

### ✅ Requirements

* Go 1.18+
* Terminal (Linux/macOS/WSL/Git Bash)

### 🚀 Running 3 Nodes (Simulated Cluster)

Run each of the following in separate terminal tabs:

```bash
# Terminal 1
$ go run ./cmd/node --port=8001 --peers=http://localhost:8002,http://localhost:8003

# Terminal 2
$ go run ./cmd/node --port=8002 --peers=http://localhost:8001,http://localhost:8003

# Terminal 3
$ go run ./cmd/node --port=8003 --peers=http://localhost:8001,http://localhost:8002
```

Each node will:

* Start an HTTP server on its assigned port
* Periodically ping its peers' /ping endpoints
* Track which peers are alive
* Expose metrics on `/metrics` endpoint

---

## 📊 Prometheus Metrics

Each node serves Prometheus-compatible metrics on `:9100/metrics`.

### Metrics Available:

* `heartbeat_pings_total`: Count of ping attempts
* `heartbeat_pings_success_total`: Successful ping responses
* `heartbeat_pings_failed_total`: Failed ping attempts

### Prometheus Config Example:

```yaml
scrape_configs:
  - job_name: 'heartbeat-nodes'
    static_configs:
      - targets: ['localhost:9100', 'localhost:9101', 'localhost:9102']
```

---

## 📈 Logging & Observability

* Uses [zap](https://github.com/uber-go/zap) for structured, leveled logs
* Log levels: `INFO`, `WARN`, `ERROR`
* Fields include peer, latency, errors, status

### Sample Logs

```json
INFO    monitor.go:42    Peer is online    {"peer": "http://localhost:8002", "lastPingSecondsAgo": 1.92}
WARN    monitor.go:31    Peer unreachable  {"peer": "http://localhost:8003", "error": "connection refused"}
```

---

## 🧩 Graceful Shutdown

Implemented using:

* `context.WithCancel()` for goroutine management
* Signal trap for `SIGINT`/`SIGTERM`
* Proper cleanup of the HTTP server

Ensures safe, clean node shutdowns.

---

## ✅ Features Summary

| Feature            | Status |
| ------------------ | ------ |
| Multi-node support | ✅      |
| Peer-to-peer pings | ✅      |
| Graceful shutdown  | ✅      |
| Zap logging        | ✅      |
| Prometheus metrics | ✅      |
| Configurable CLI   | ✅      |

---

## 📚 Educational Value

This project lays the foundation for understanding:

* Microservice communication patterns
* Fault detection algorithms
* Metrics-driven observability
* Concurrent system design in Go

A great stepping stone before diving into Raft, Consul, etcd, or Kubernetes internals.

---

## 👨‍💻 Author

Created by [YpatiosCh](https://github.com/YpatiosCh) as part of a series to learn real-world distributed systems using modern Go.

---

## 🪪 License

Licensed under the MIT License. See `LICENSE` file.
