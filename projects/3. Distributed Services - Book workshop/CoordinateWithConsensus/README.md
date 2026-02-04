# Chapter 9: Coordinate Your Services with Consensus

## Overview

This chapter implements the Raft consensus algorithm to create a replicated, strongly consistent distributed log. You'll learn how to achieve consensus across multiple servers, handle leader election, and replicate data reliably.

## What You'll Learn

- Understanding consensus and the CAP theorem
- Implementing Raft consensus algorithm
- Building a replicated log with strong consistency
- Handling leader election and failover
- Managing log replication across servers
- Testing distributed consensus systems

## Consensus Fundamentals

### The Problem

Multiple servers need to agree on the same sequence of operations (the log) despite:
- Network failures
- Server crashes
- Delayed messages
- Out-of-order delivery

### CAP Theorem

You can have at most 2 of 3:
- **Consistency** - All nodes see the same data
- **Availability** - Every request gets a response
- **Partition Tolerance** - System works despite network splits

Raft chooses **CP** (Consistency + Partition Tolerance)

### Why Raft?

Compared to alternatives like Paxos:
- ✅ Easier to understand
- ✅ Proven correctness
- ✅ Leader-based (simplifies logic)
- ✅ Battle-tested implementations
- ✅ Strong consistency guarantees

## Raft Overview

### Core Concepts

**Leader Election**
- One server is elected leader
- Leader handles all client requests
- Followers redirect to leader
- New election if leader fails

**Log Replication**
- Leader receives client requests
- Leader appends to its log
- Leader replicates to followers
- Commit when majority have entry

**Safety**
- Only committed entries returned to clients
- Committed entries never lost
- Logs remain consistent

### Server States

```
┌──────────┐
│ Follower │ ←──────────┐
└──────────┘            │
     │                  │
     │ timeout          │ discovers leader
     │                  │ or new term
     ▼                  │
┌──────────┐            │
│Candidate │ ───────────┤
└──────────┘  election  │
     │        timeout   │
     │ receives         │
     │ majority votes   │
     ▼                  │
┌──────────┐            │
│  Leader  │ ───────────┘
└──────────┘  step down
              (higher term)
```

## Project Structure

```
CoordinateWithConsensus/
├── api/
│   └── v1/
│       ├── log.proto
│       ├── log.pb.go
│       └── log_grpc.pb.go
├── internal/
│   ├── log/
│   │   ├── distributed.go    # Raft-backed log
│   │   ├── distributed_test.go
│   │   ├── replicator.go     # Handles replication
│   │   ├── config.go
│   │   └── ...
│   ├── discovery/
│   │   └── ...
│   └── server/
│       └── ...
├── go.mod
└── go.sum
```

## Distributed Log Implementation

### Raft-Backed Log

```go
type DistributedLog struct {
    config Config
    log    *Log
    raft   *raft.Raft
}

type Config struct {
    Raft struct {
        raft.Config
        StreamLayer *StreamLayer
        Bootstrap   bool
    }
    Segment struct {
        MaxStoreBytes uint64
        MaxIndexBytes uint64
        InitialOffset uint64
    }
}
```

### Creating a Distributed Log

```go
func NewDistributedLog(dataDir string, config Config) (
    *DistributedLog,
    error,
) {
    l := &DistributedLog{
        config: config,
    }
    
    if err := l.setupLog(dataDir); err != nil {
        return nil, err
    }
    
    if err := l.setupRaft(dataDir); err != nil {
        return nil, err
    }
    
    return l, nil
}
```

### Setting Up Raft

```go
func (l *DistributedLog) setupRaft(dataDir string) error {
    // Create FSM (Finite State Machine)
    fsm := &fsm{log: l.log}
    
    // Create log store
    logDir := filepath.Join(dataDir, "raft", "log")
    if err := os.MkdirAll(logDir, 0755); err != nil {
        return err
    }
    logConfig := l.config
    logConfig.Segment.InitialOffset = 1
    logStore, err := newLogStore(logDir, logConfig)
    if err != nil {
        return err
    }
    
    // Create stable store (for Raft metadata)
    stableStore, err := raftboltdb.NewBoltStore(
        filepath.Join(dataDir, "raft", "stable"),
    )
    if err != nil {
        return err
    }
    
    // Create snapshot store
    retain := 1
    snapshotStore, err := raft.NewFileSnapshotStore(
        filepath.Join(dataDir, "raft"),
        retain,
        os.Stderr,
    )
    if err != nil {
        return err
    }
    
    // Create transport
    maxPool := 5
    timeout := 10 * time.Second
    transport := raft.NewNetworkTransport(
        l.config.Raft.StreamLayer,
        maxPool,
        timeout,
        os.Stderr,
    )
    
    // Create Raft
    config := raft.DefaultConfig()
    config.LocalID = l.config.Raft.LocalID
    if l.config.Raft.HeartbeatTimeout != 0 {
        config.HeartbeatTimeout = l.config.Raft.HeartbeatTimeout
    }
    if l.config.Raft.ElectionTimeout != 0 {
        config.ElectionTimeout = l.config.Raft.ElectionTimeout
    }
    if l.config.Raft.LeaderLeaseTimeout != 0 {
        config.LeaderLeaseTimeout = l.config.Raft.LeaderLeaseTimeout
    }
    if l.config.Raft.CommitTimeout != 0 {
        config.CommitTimeout = l.config.Raft.CommitTimeout
    }
    
    l.raft, err = raft.NewRaft(
        config,
        fsm,
        logStore,
        stableStore,
        snapshotStore,
        transport,
    )
    if err != nil {
        return err
    }
    
    // Bootstrap if first node
    if l.config.Raft.Bootstrap {
        config := raft.Configuration{
            Servers: []raft.Server{{
                ID:      config.LocalID,
                Address: transport.LocalAddr(),
            }},
        }
        err = l.raft.BootstrapCluster(config).Error()
    }
    
    return err
}
```

## Finite State Machine (FSM)

The FSM defines how Raft commands modify state:

```go
type fsm struct {
    log *Log
}

// Apply is called by Raft when a log entry is committed
func (f *fsm) Apply(record *raft.Log) interface{} {
    buf := record.Data
    reqType := RequestType(buf[0])
    
    switch reqType {
    case AppendRequestType:
        return f.applyAppend(buf[1:])
    }
    
    return nil
}

func (f *fsm) applyAppend(b []byte) interface{} {
    var req api.ProduceRequest
    err := proto.Unmarshal(b, &req)
    if err != nil {
        return err
    }
    
    offset, err := f.log.Append(req.Record)
    if err != nil {
        return err
    }
    
    return &api.ProduceResponse{Offset: offset}
}

// Snapshot captures current state
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
    r := f.log.Reader()
    return &snapshot{reader: r}, nil
}

// Restore restores from snapshot
func (f *fsm) Restore(r io.ReadCloser) error {
    b := make([]byte, lenWidth)
    var buf bytes.Buffer
    
    for {
        _, err := io.ReadFull(r, b)
        if err == io.EOF {
            break
        } else if err != nil {
            return err
        }
        
        size := int64(enc.Uint64(b))
        if _, err = io.CopyN(&buf, r, size); err != nil {
            return err
        }
        
        record := &api.Record{}
        if err = proto.Unmarshal(buf.Bytes(), record); err != nil {
            return err
        }
        
        if _, err = f.log.Append(record); err != nil {
            return err
        }
        
        buf.Reset()
    }
    
    return nil
}
```

## Log Operations

### Append (Write)

```go
func (l *DistributedLog) Append(record *api.Record) (uint64, error) {
    // Serialize request
    res, err := l.apply(
        AppendRequestType,
        &api.ProduceRequest{Record: record},
    )
    if err != nil {
        return 0, err
    }
    return res.(*api.ProduceResponse).Offset, nil
}

func (l *DistributedLog) apply(reqType RequestType, req proto.Message) (
    interface{},
    error,
) {
    var buf bytes.Buffer
    _, err := buf.Write([]byte{byte(reqType)})
    if err != nil {
        return nil, err
    }
    
    b, err := proto.Marshal(req)
    if err != nil {
        return nil, err
    }
    _, err = buf.Write(b)
    if err != nil {
        return nil, err
    }
    
    // Apply goes through Raft
    timeout := 10 * time.Second
    future := l.raft.Apply(buf.Bytes(), timeout)
    if future.Error() != nil {
        return nil, future.Error()
    }
    
    res := future.Response()
    if err, ok := res.(error); ok {
        return nil, err
    }
    
    return res, nil
}
```

### Read

```go
func (l *DistributedLog) Read(offset uint64) (*api.Record, error) {
    return l.log.Read(offset)
}
```

Reads are served locally (no Raft consensus needed).

## Cluster Management

### Joining a Server

```go
func (l *DistributedLog) Join(id, addr string) error {
    configFuture := l.raft.GetConfiguration()
    if err := configFuture.Error(); err != nil {
        return err
    }
    
    serverID := raft.ServerID(id)
    serverAddr := raft.ServerAddress(addr)
    
    // Check if already a member
    for _, srv := range configFuture.Configuration().Servers {
        if srv.ID == serverID || srv.Address == serverAddr {
            // Already a member
            if srv.ID == serverID && srv.Address == serverAddr {
                return nil
            }
            // Remove stale entry
            removeFuture := l.raft.RemoveServer(serverID, 0, 0)
            if err := removeFuture.Error(); err != nil {
                return err
            }
        }
    }
    
    // Add new server
    addFuture := l.raft.AddVoter(serverID, serverAddr, 0, 0)
    if err := addFuture.Error(); err != nil {
        return err
    }
    
    return nil
}
```

### Leaving a Server

```go
func (l *DistributedLog) Leave(id string) error {
    removeFuture := l.raft.RemoveServer(raft.ServerID(id), 0, 0)
    return removeFuture.Error()
}
```

## Stream Layer (Network Transport)

Raft needs a transport layer:

```go
type StreamLayer struct {
    ln              net.Listener
    serverTLSConfig *tls.Config
    peerTLSConfig   *tls.Config
}

func NewStreamLayer(
    ln net.Listener,
    serverTLSConfig,
    peerTLSConfig *tls.Config,
) *StreamLayer {
    return &StreamLayer{
        ln:              ln,
        serverTLSConfig: serverTLSConfig,
        peerTLSConfig:   peerTLSConfig,
    }
}

func (s *StreamLayer) Dial(
    addr raft.ServerAddress,
    timeout time.Duration,
) (net.Conn, error) {
    dialer := &net.Dialer{Timeout: timeout}
    
    var conn, err = dialer.Dial("tcp", string(addr))
    if err != nil {
        return nil, err
    }
    
    // Identify as Raft connection
    _, err = conn.Write([]byte{RaftRPC})
    if err != nil {
        return nil, err
    }
    
    if s.peerTLSConfig != nil {
        conn = tls.Client(conn, s.peerTLSConfig)
    }
    
    return conn, err
}

func (s *StreamLayer) Accept() (net.Conn, error) {
    conn, err := s.ln.Accept()
    if err != nil {
        return nil, err
    }
    
    b := make([]byte, 1)
    _, err = conn.Read(b)
    if err != nil {
        return nil, err
    }
    
    if bytes.Compare([]byte{RaftRPC}, b) != 0 {
        return nil, fmt.Errorf("not a raft rpc")
    }
    
    if s.serverTLSConfig != nil {
        return tls.Server(conn, s.serverTLSConfig), nil
    }
    
    return conn, nil
}
```

## Testing Consensus

### Multi-Node Test

```go
func TestMultipleServers(t *testing.T) {
    var logs []*DistributedLog
    
    // Create 3 servers
    nodeCount := 3
    ports := dynaport.Get(nodeCount)
    
    for i := 0; i < nodeCount; i++ {
        dataDir, err := ioutil.TempDir("", "distributed-log-test")
        require.NoError(t, err)
        defer os.RemoveAll(dataDir)
        
        ln, err := net.Listen(
            "tcp",
            fmt.Sprintf("127.0.0.1:%d", ports[i]),
        )
        require.NoError(t, err)
        
        config := Config{}
        config.Raft.StreamLayer = NewStreamLayer(ln, nil, nil)
        config.Raft.LocalID = raft.ServerID(fmt.Sprintf("%d", i))
        config.Raft.HeartbeatTimeout = 50 * time.Millisecond
        config.Raft.ElectionTimeout = 50 * time.Millisecond
        config.Raft.LeaderLeaseTimeout = 50 * time.Millisecond
        config.Raft.CommitTimeout = 5 * time.Millisecond
        
        if i == 0 {
            config.Raft.Bootstrap = true
        }
        
        l, err := NewDistributedLog(dataDir, config)
        require.NoError(t, err)
        
        if i != 0 {
            err = logs[0].Join(
                fmt.Sprintf("%d", i), ln.Addr().String(),
            )
            require.NoError(t, err)
        }
        
        logs = append(logs, l)
    }
    
    // Wait for leader election
    time.Sleep(3 * time.Second)
    
    // Append to leader
    record := &api.Record{Value: []byte("first")}
    off, err := logs[0].Append(record)
    require.NoError(t, err)
    require.Equal(t, uint64(0), off)
    
    // Wait for replication
    time.Sleep(1 * time.Second)
    
    // Verify replicated to all servers
    for i, log := range logs {
        got, err := log.Read(off)
        require.NoError(t, err)
        require.Equal(t, record.Value, got.Value, "server %d", i)
    }
}
```

## Leader Election

### Election Process

1. **Follower timeout** - No heartbeat from leader
2. **Become candidate** - Increment term, vote for self
3. **Request votes** - Send RequestVote RPCs to all servers
4. **Receive votes** - Need majority to win
5. **Become leader** - If won, start sending heartbeats
6. **Revert to follower** - If lost or discovered higher term

### Configuration

```go
config := raft.DefaultConfig()

// How long follower waits before becoming candidate
config.HeartbeatTimeout = 1000 * time.Millisecond

// How long candidate waits before new election
config.ElectionTimeout = 1000 * time.Millisecond

// How long leader lease lasts
config.LeaderLeaseTimeout = 500 * time.Millisecond
```

## Snapshots

Snapshots compact the log:

```go
// Create snapshot
future := l.raft.Snapshot()
if err := future.Error(); err != nil {
    return err
}

// Restore from snapshot (automatic on start)
// Raft calls fsm.Restore()
```

**Benefits:**
- Faster recovery
- Bounded disk usage
- Quicker catch-up for lagging followers

## Read Consistency Levels

### Stale Read (Fast)
```go
// Read from local log (might be stale)
record, err := log.Read(offset)
```

### Linearizable Read (Slow)
```go
// Verify still leader before reading
verifyFuture := l.raft.VerifyLeader()
if err := verifyFuture.Error(); err != nil {
    return nil, err  // Not leader or can't confirm
}
record, err := log.Read(offset)
```

Trade-off: Consistency vs Latency

## Best Practices

### Configuration
- ✅ Odd number of servers (3, 5, 7)
- ✅ Tune timeouts for network latency
- ✅ Use persistent storage
- ✅ Regular snapshots
- ✅ Monitor Raft metrics

### Operations
- ✅ Graceful server replacement
- ✅ Test failure scenarios
- ✅ Monitor leader elections
- ✅ Watch for split-brain
- ✅ Plan for disaster recovery

### Performance
- ✅ Batch writes when possible
- ✅ Use pipelining
- ✅ Compress snapshots
- ✅ Tune fsync settings
- ✅ Monitor replication lag

## Key Takeaways

✅ Raft provides strong consistency  
✅ Leader handles all writes (simplifies logic)  
✅ Majority required for commits (fault tolerance)  
✅ Automatic leader election (no SPOF)  
✅ Snapshots keep log bounded  

## Next Steps

Move to **Chapter 10: Deploy to Cloud** to learn how to:
- Package services in Docker containers
- Deploy to Kubernetes
- Use Helm for management
- Run in production environments

## Dependencies

```
github.com/hashicorp/raft v1.7.1                    # Raft consensus
github.com/hashicorp/raft-boltdb v0.0.0-20230125... # Raft stable store
```

## Additional Resources

- [Raft Paper](https://raft.github.io/raft.pdf)
- [Raft Visualization](http://thesecretlivesofdata.com/raft/)
- [HashiCorp Raft Library](https://github.com/hashicorp/raft)
- [Raft FAQ](https://raft.github.io/)

---

**Ready for production?** The final chapter deploys your service to Kubernetes! ☸️