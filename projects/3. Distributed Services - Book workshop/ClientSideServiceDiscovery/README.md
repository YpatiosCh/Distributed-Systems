# Chapter 8: Client-Side Service Discovery with Serf

## Overview

This chapter implements client-side service discovery using HashiCorp's Serf for gossip-based cluster membership. You'll learn how nodes discover each other automatically, detect failures, and maintain cluster state without centralized coordination.

## What You'll Learn

- Understanding gossip protocols (SWIM)
- Implementing cluster membership with Serf
- Handling node joins and leaves
- Failure detection and recovery
- Event-driven cluster updates
- Comparing client-side vs server-side discovery

## Client-Side vs Server-Side Discovery

### Server-Side (Chapter 7)
```
Client → Load Balancer → Server
```
- Centralized routing
- Simple client
- Load balancer is SPOF
- Extra network hop

### Client-Side (This Chapter)
```
Client → [Discovers servers] → Direct connection to Server
```
- Distributed discovery
- Smart client
- No SPOF
- Direct connections

## Serf Overview

**Serf** is a decentralized solution for cluster membership, failure detection, and orchestration based on the SWIM (Scalable Weakly-consistent Infection-style Process Group Membership) protocol.

### Key Features

- **Gossip-based** - Information spreads organically
- **Eventually consistent** - All nodes converge to same state
- **Failure detection** - Quickly detects node failures
- **Event propagation** - Custom events across cluster
- **No single point of failure**
- **Lightweight** - Minimal overhead

### How Gossip Works

```
Node A knows: {A, B}
Node B knows: {A, B, C}
Node C knows: {B, C, D}

After gossip:
All nodes know: {A, B, C, D}
```

Nodes periodically exchange state with random neighbors, ensuring information eventually reaches all nodes.

## Project Structure

```
ClientSideServiceDiscovery/
├── api/
│   └── v1/
│       └── ...
├── internal/
│   ├── discovery/
│   │   ├── membership.go     # Serf membership management
│   │   └── membership_test.go
│   ├── log/
│   │   └── ...
│   ├── agent/
│   │   ├── agent.go          # Combines log + discovery
│   │   └── agent_test.go
│   └── server/
│       └── ...
├── go.mod
└── go.sum
```

## Membership Implementation

### Membership Structure

```go
type Membership struct {
    Config
    handler Handler
    serf    *serf.Serf
    events  chan serf.Event
    logger  *zap.Logger
}

type Config struct {
    NodeName       string
    BindAddr       string
    Tags           map[string]string
    StartJoinAddrs []string
}

type Handler interface {
    Join(name, addr string) error
    Leave(name string) error
}
```

### Creating a Membership

```go
func New(handler Handler, config Config) (*Membership, error) {
    m := &Membership{
        Config:  config,
        handler: handler,
        logger:  zap.L().Named("membership"),
    }
    
    if err := m.setupSerf(); err != nil {
        return nil, err
    }
    
    return m, nil
}
```

### Setting Up Serf

```go
func (m *Membership) setupSerf() error {
    addr, err := net.ResolveTCPAddr("tcp", m.BindAddr)
    if err != nil {
        return err
    }
    
    config := serf.DefaultConfig()
    config.Init()
    config.MemberlistConfig.BindAddr = addr.IP.String()
    config.MemberlistConfig.BindPort = addr.Port
    config.NodeName = m.NodeName
    config.Tags = m.Tags
    
    m.events = make(chan serf.Event)
    config.EventCh = m.events
    
    m.serf, err = serf.Create(config)
    if err != nil {
        return err
    }
    
    // Join existing cluster
    if len(m.StartJoinAddrs) > 0 {
        if _, err := m.serf.Join(m.StartJoinAddrs, true); err != nil {
            return err
        }
    }
    
    // Start event handler
    go m.eventHandler()
    
    return nil
}
```

### Handling Membership Events

```go
func (m *Membership) eventHandler() {
    for e := range m.events {
        switch e.EventType() {
        case serf.EventMemberJoin:
            for _, member := range e.(serf.MemberEvent).Members {
                if m.isLocal(member) {
                    continue
                }
                m.handleJoin(member)
            }
        case serf.EventMemberLeave, serf.EventMemberFailed:
            for _, member := range e.(serf.MemberEvent).Members {
                if m.isLocal(member) {
                    continue
                }
                m.handleLeave(member)
            }
        }
    }
}

func (m *Membership) handleJoin(member serf.Member) {
    if err := m.handler.Join(
        member.Name,
        member.Tags["rpc_addr"],
    ); err != nil {
        m.logError(err, "failed to join", member)
    }
}

func (m *Membership) handleLeave(member serf.Member) {
    if err := m.handler.Leave(
        member.Name,
    ); err != nil {
        m.logError(err, "failed to leave", member)
    }
}

func (m *Membership) isLocal(member serf.Member) bool {
    return m.serf.LocalMember().Name == member.Name
}
```

### Querying Cluster Members

```go
func (m *Membership) Members() []serf.Member {
    return m.serf.Members()
}

func (m *Membership) Leave() error {
    return m.serf.Leave()
}
```

## Agent: Combining Log + Discovery

The agent ties together the log service and cluster membership:

```go
type Agent struct {
    Config
    
    log        *log.Log
    server     *grpc.Server
    membership *discovery.Membership
    
    shutdown     bool
    shutdowns    chan struct{}
    shutdownLock sync.Mutex
}

type Config struct {
    ServerTLSConfig *tls.Config
    PeerTLSConfig   *tls.Config
    DataDir         string
    BindAddr        string
    RPCPort         int
    NodeName        string
    StartJoinAddrs  []string
    ACLModelFile    string
    ACLPolicyFile   string
}
```

### Creating an Agent

```go
func New(config Config) (*Agent, error) {
    a := &Agent{
        Config:    config,
        shutdowns: make(chan struct{}),
    }
    
    setup := []func() error{
        a.setupLogger,
        a.setupLog,
        a.setupServer,
        a.setupMembership,
    }
    
    for _, fn := range setup {
        if err := fn(); err != nil {
            return nil, err
        }
    }
    
    return a, nil
}
```

### Setting Up Components

```go
func (a *Agent) setupLog() error {
    var err error
    a.log, err = log.NewLog(
        a.Config.DataDir,
        log.Config{},
    )
    return err
}

func (a *Agent) setupServer() error {
    var err error
    a.server, err = server.NewGRPCServer(&server.Config{
        CommitLog: a.log,
        // ... other config
    })
    return err
}

func (a *Agent) setupMembership() error {
    var err error
    a.membership, err = discovery.New(a, discovery.Config{
        NodeName: a.Config.NodeName,
        BindAddr: a.Config.BindAddr,
        Tags: map[string]string{
            "rpc_addr": fmt.Sprintf("%s:%d", 
                a.Config.BindAddr, 
                a.Config.RPCPort,
            ),
        },
        StartJoinAddrs: a.Config.StartJoinAddrs,
    })
    return err
}
```

### Implementing Handler Interface

```go
func (a *Agent) Join(name, addr string) error {
    a.log.Info("node joined",
        zap.String("name", name),
        zap.String("addr", addr),
    )
    // Could add replication here
    return nil
}

func (a *Agent) Leave(name string) error {
    a.log.Info("node left",
        zap.String("name", name),
    )
    // Could handle replication cleanup here
    return nil
}
```

## Running a Cluster

### Starting First Node

```go
agent1, err := agent.New(agent.Config{
    DataDir:   "/tmp/agent1",
    NodeName:  "node1",
    BindAddr:  "127.0.0.1:8401",
    RPCPort:   8400,
})
```

### Starting Second Node (Joins First)

```go
agent2, err := agent.New(agent.Config{
    DataDir:        "/tmp/agent2",
    NodeName:       "node2",
    BindAddr:       "127.0.0.1:8501",
    RPCPort:        8500,
    StartJoinAddrs: []string{"127.0.0.1:8401"},  // Join node1
})
```

### Starting Third Node

```go
agent3, err := agent.New(agent.Config{
    DataDir:        "/tmp/agent3",
    NodeName:       "node3",
    BindAddr:       "127.0.0.1:8601",
    RPCPort:        8600,
    StartJoinAddrs: []string{"127.0.0.1:8401"},  // Join node1
})
```

All nodes discover each other through gossip!

## Failure Detection

### SWIM Protocol

Serf uses SWIM for failure detection:

1. **Ping** - Node A pings Node B
2. **Ack** - Node B responds (healthy)
3. **No Ack** - Node B doesn't respond
4. **Indirect Ping** - Node A asks Node C to ping Node B
5. **Suspect** - If still no response, mark suspect
6. **Confirmed Dead** - After timeout, mark dead

### Configuration

```go
config := serf.DefaultConfig()

// How long to wait for ping response
config.MemberlistConfig.ProbeInterval = 1 * time.Second

// How many consecutive failures before marking dead
config.MemberlistConfig.ProbeTimeout = 500 * time.Millisecond

// How long to wait in suspect state
config.MemberlistConfig.SuspicionMult = 3
```

## Node Tags

Tags attach metadata to nodes:

```go
config.Tags = map[string]string{
    "rpc_addr": "10.0.1.5:8400",
    "role":     "follower",
    "dc":       "us-east-1",
    "version":  "1.0.0",
}
```

Other nodes can query tags:

```go
for _, member := range membership.Members() {
    rpcAddr := member.Tags["rpc_addr"]
    role := member.Tags["role"]
    // Connect to rpcAddr...
}
```

## Custom Events

Serf supports custom events:

```go
// Send event
err := serf.UserEvent(
    "deploy",
    []byte("v1.2.3"),
    false, // coalesce
)

// Handle event
case serf.EventUser:
    event := e.(serf.UserEvent)
    if event.Name == "deploy" {
        version := string(event.Payload)
        // Handle deployment...
    }
}
```

## Testing

### Test Setup

```go
func TestAgent(t *testing.T) {
    var agents []*agent.Agent
    
    // Start 3 agents
    for i := 0; i < 3; i++ {
        ports := dynaport.Get(2)
        bindAddr := fmt.Sprintf("127.0.0.1:%d", ports[0])
        rpcPort := ports[1]
        
        dataDir, err := ioutil.TempDir("", "agent-test")
        require.NoError(t, err)
        
        var startJoinAddrs []string
        if i != 0 {
            startJoinAddrs = append(
                startJoinAddrs,
                agents[0].Config.BindAddr,
            )
        }
        
        agent, err := agent.New(agent.Config{
            NodeName:       fmt.Sprintf("%d", i),
            StartJoinAddrs: startJoinAddrs,
            BindAddr:       bindAddr,
            RPCPort:        rpcPort,
            DataDir:        dataDir,
        })
        require.NoError(t, err)
        
        agents = append(agents, agent)
    }
    
    defer func() {
        for _, agent := range agents {
            err := agent.Shutdown()
            require.NoError(t, err)
        }
    }()
    
    // Wait for cluster to form
    time.Sleep(3 * time.Second)
    
    // Verify all nodes know about each other
    for _, agent := range agents {
        require.Equal(t, 3, len(agent.membership.Members()))
    }
}
```

## Advantages of Client-Side Discovery

✅ **No single point of failure** - Decentralized  
✅ **Direct connections** - No extra hop  
✅ **Fast failure detection** - Gossip protocol  
✅ **Automatic recovery** - Nodes rejoin automatically  
✅ **Low overhead** - Efficient gossip  

## Disadvantages

❌ **Smart clients** - More complex client logic  
❌ **Eventually consistent** - Brief inconsistencies possible  
❌ **Client updates** - Must update all clients for changes  
❌ **More network traffic** - Gossip overhead  

## When to Use Each

### Use Client-Side When:
- Low latency is critical
- You control all clients
- No central infrastructure preferred
- Failure detection speed matters

### Use Server-Side When:
- Clients are external/untrusted
- Centralized control needed
- Clients are diverse (mobile, web, etc.)
- Simpler client logic preferred

## Best Practices

### Configuration
- ✅ Use appropriate timeouts for your network
- ✅ Set reasonable gossip intervals
- ✅ Use tags for routing information
- ✅ Test failure scenarios

### Operations
- ✅ Monitor cluster health
- ✅ Log membership changes
- ✅ Handle split-brain scenarios
- ✅ Implement graceful shutdown

### Security
- ✅ Use Serf encryption
- ✅ Authenticate cluster members
- ✅ Limit who can join cluster
- ✅ Monitor for unexpected nodes

## Key Takeaways

✅ Gossip protocols enable decentralized discovery  
✅ Serf provides robust cluster membership  
✅ Client-side discovery offers low latency  
✅ Trade-off between consistency and availability  
✅ Choose pattern based on requirements  

## Next Steps

Move to **Chapter 9: Coordinate with Consensus** to learn how to:
- Implement the Raft consensus algorithm
- Replicate logs across servers
- Handle leader election
- Achieve strong consistency

## Dependencies

```
github.com/hashicorp/serf v0.10.1              # Cluster membership
github.com/travisjeffery/go-dynaport v1.0.0    # Dynamic port allocation (testing)
```

## Additional Resources

- [Serf Documentation](https://www.serf.io/docs/)
- [SWIM Protocol Paper](https://www.cs.cornell.edu/projects/Quicksilver/public_pdfs/SWIM.pdf)
- [Gossip Protocols](https://en.wikipedia.org/wiki/Gossip_protocol)
- [Eventually Consistent](https://www.allthingsdistributed.com/2008/12/eventually_consistent.html)

---

**Ready for strong consistency?** The next chapter implements Raft consensus for replicated logs! 🎯