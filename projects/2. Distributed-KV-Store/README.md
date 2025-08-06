# Distributed Key-Value Store (Course-Like Guide)

## Table of Contents

1. [Introduction](#introduction)
2. [Project Overview](#project-overview)
3. [Configuration](#configuration)
4. [How It Works](#how-it-works)
5. [Running the Project](#running-the-project)
6. [API Endpoints](#api-endpoints)
7. [Replication Logic](#replication-logic)
8. [Advanced Features](#advanced-features)
9. [Contributing](#contributing)

---

## Introduction

Welcome to the **Distributed Key-Value Store** project. This is a simple but powerful distributed key-value store that ensures fault tolerance and consistency across multiple nodes. The system replicates key-value pairs across peers and maintains the state of the nodes in a network.

This project is designed to demonstrate how to build a basic **distributed system** with **replication** and **peer synchronization**.

By the end of this guide, you will have a working distributed key-value store that can:

* Store key-value pairs locally and across peers.
* Replicate data between nodes when necessary.
* Handle node failure and re-synchronization when a peer comes back online.

---

## Project Overview

The **Distributed Key-Value Store** is implemented using **Golang** and includes the following key features:

* **Peer Communication**: Nodes (peers) communicate over HTTP.
* **Periodic Pinging**: Nodes regularly ping each other to check if they are online and synchronize the stores if needed.
* **Store Replication**: When a node comes online after failure, its store is synchronized with the current node's store.
* **Fault Tolerance**: When a node goes down, the system continues to function, and the store will sync once the node is back.

---

## Configuration

Before running the project, you'll need to configure the **node** settings. The configuration is done through **command-line flags** when running each node. Here’s how to use them:

### Command-Line Flags:

```bash
--port=8001          # The port on which the node will run (required)
--peers=...          # Comma-separated list of peers in the format http://localhost:port (required)
--pingfreq=10        # Frequency (in seconds) to ping peers (optional)
--timeout=15         # Timeout (in seconds) for HTTP requests (optional)
```

### Example Usage:

#### 1. **Node 1** (Port: 8001):

```bash
go run cmd/srv/main.go --port=8001 --peers=http://localhost:8002,http://localhost:8003 --pingfreq=10 --timeout=15
```

#### 2. **Node 2** (Port: 8002):

```bash
go run cmd/srv/main.go --port=8002 --peers=http://localhost:8001,http://localhost:8003 --pingfreq=10 --timeout=15
```

#### 3. **Node 3** (Port: 8003):

```bash
go run cmd/srv/main.go --port=8003 --peers=http://localhost:8001,http://localhost:8002 --pingfreq=10 --timeout=15
```

Each node communicates with the others in the list of peers. Make sure the peers are running before starting the nodes.

---

## How It Works

The system is composed of nodes (peers) that can store, replicate, and synchronize data. Here’s a step-by-step guide to the system’s workflow:

### 1. **Node Initialization**:

* When a node starts, it initializes its store and begins to periodically **ping** its peers to check if they are online.
* Each node runs a web server that exposes several HTTP endpoints (`/ping`, `/store`, `/replicate`, `/store/hash`, `/store/key`).

### 2. **Peer Communication**:

* Every node checks if its peers are online by sending a **ping request** to each peer at regular intervals (`PingFrequency`).
* If a peer is down, the node will mark it as such and wait until the peer is back online.

### 3. **Replicating Stores**:

* If a peer goes down and comes back online, the node compares the local store with the peer's store. If the stores are different, the node will replicate its store to the peer.
* The comparison is done using a **hash** of the store. If the hashes don’t match, the entire store is replicated.

### 4. **Store Hashing**:

* Each node computes a hash of its store to efficiently compare with the peer's store. If the hashes differ, replication is triggered.
* The store is represented as an array of `key-value` pairs.

---

## Running the Project

### 1. **Running a Single Node**:

To run a node, execute the following command with the desired flags:

```bash
go run cmd/srv/main.go --port=<port> --peers=<comma-separated-peers> --pingfreq=<ping-frequency> --timeout=<timeout>
```

This will start a node on the port specified in your flags. The node will start its HTTP server and begin pinging the peers.

### 2. **Running Multiple Nodes**:

To simulate a distributed system, run multiple nodes with different configurations. For example:

* Node 1 (Port: 8001):

  ```bash
  go run cmd/srv/main.go --port=8001 --peers=http://localhost:8002,http://localhost:8003 --pingfreq=10 --timeout=15
  ```

* Node 2 (Port: 8002):

  ```bash
  go run cmd/srv/main.go --port=8002 --peers=http://localhost:8001,http://localhost:8003 --pingfreq=10 --timeout=15
  ```

* Node 3 (Port: 8003):

  ```bash
  go run cmd/srv/main.go --port=8003 --peers=http://localhost:8001,http://localhost:8002 --pingfreq=10 --timeout=15
  ```

Each node will communicate with the others in the peer list. Make sure the peers are running before starting each node.

---

## API Endpoints

### 1. **`GET /ping`**:

* This endpoint responds with a simple `"pong"` message to indicate that the node is up.

**Example Request**:

```bash
curl http://localhost:8001/ping
```

**Response**:

```json
{"message": "pong"}
```

### 2. **`POST /store`**:

* This endpoint stores a key-value pair in the local store and replicates it to all peers.

**Example Request**:

```bash
curl -X POST http://localhost:8001/store -d '{"key": "hello", "value": "world"}' -H "Content-Type: application/json"
```

### 3. **`POST /replicate`**:

* This endpoint is used by peers to replicate a key-value pair. It expects a `POST` request with a key-value pair in the body.


### 4. **`GET /store/hash`**:

* This endpoint returns the hash of the local store. It is used by peers to check if their stores are in sync.

**Example Request**:

```bash
curl http://localhost:8001/store/hash
```

**Response**:

```json
{"hash": "abc123"}
```

### 5. **`GET /store/key`**:

* This endpoint retrieves the value for a given key from the local store.

**Example Request**:

```bash
curl -X GET http://localhost:8001/store/key -d '{"key": "hello"}' -H "Content-Type: application/json"
```

**Response (if key exists)**:

```json
{"value": "world"}
```

**Response (if key does not exist)**:

```json
{"error": "Key not found"}
```

---

## Replication Logic

1. **Periodically Pinging Peers**: Each node pings its peers at regular intervals (`PingFrequency`) to check if they are online.

2. **Hash Comparison**: When a peer comes back online, the node compares the hash of the peer's store with its own. If the hashes differ, the entire store is replicated.

3. **Efficient Replication**: If the stores are already in sync (i.e., the hashes match), no replication occurs. This ensures that unnecessary data transfer is avoided.

---

## Advanced Features

1. **Peer Failure Detection**:

   * Nodes are capable of detecting peer failures based on the response from the ping request. If a peer does not respond within the specified `Timeout`, it is considered down.
2. **Fault Tolerance**:

   * The system continues to function even if one or more peers are down. When a peer comes back online, it will synchronize its store with the


node.

3. **Dynamic Peer List**:

   * You can modify the configuration to add/remove peers dynamically, although you would need to restart the nodes to pick up the new configuration.

---

## Contributing

We welcome contributions to improve this project! Feel free to open an issue or submit a pull request. When contributing, please follow the standard GitHub practices for creating clear and descriptive commits.

---

### **That's it!** 🎉

You've now completed the guide for setting up and running your **Distributed Key-Value Store**. You should have a working distributed system that replicates data between peers, checks the store's consistency, and ensures fault tolerance in the system.
