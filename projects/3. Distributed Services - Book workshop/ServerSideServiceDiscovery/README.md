# Chapter 7: Server-Side Service Discovery

## Overview

This chapter implements server-side service discovery using DNS-based routing and load balancing. You'll learn how to make services discoverable, route requests to healthy instances, and implement different load balancing strategies.

## What You'll Learn

- Understanding service discovery patterns
- Implementing DNS-based service discovery
- Building custom gRPC resolvers and pickers
- Load balancing strategies (round-robin, weighted, etc.)
- Health checking and failure detection
- Testing load balancing behavior

## Service Discovery Patterns

### Client-Side Discovery
```
Client → [Service Registry] → Choose Server → Call Server
```
- Client queries registry
- Client chooses server
- Client makes direct connection
- Examples: Consul, Eureka

### Server-Side Discovery
```
Client → [Load Balancer] → Route to Server
```
- Client calls load balancer
- Load balancer chooses server
- Transparent to client
- Examples: Kubernetes Service, AWS ELB

This chapter focuses on **server-side** discovery.

## Project Structure

```
ServerSideServiceDiscovery/
├── api/
│   └── v1/
│       ├── log.proto
│       ├── log.pb.go
│       └── log_grpc.pb.go
├── internal/
│   ├── discovery/
│   │   ├── resolver.go       # gRPC resolver implementation
│   │   └── resolver_test.go
│   ├── loadbalance/
│   │   ├── picker.go         # Custom load balancer
│   │   └── picker_test.go
│   ├── log/
│   │   └── ...
│   └── server/
│       ├── server.go
│       └── server_test.go
├── go.mod
└── go.sum
```

## DNS-Based Service Discovery

### Kubernetes Service Example

When you deploy to Kubernetes:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: proglog
spec:
  ports:
  - port: 8080
    targetPort: 8080
  selector:
    app: proglog
```

DNS resolves `proglog:8080` to multiple pod IPs:
```
proglog.default.svc.cluster.local
  ↓
10.244.1.5:8080
10.244.2.3:8080
10.244.3.7:8080
```

## gRPC Name Resolution

### The Resolution Process

1. **Client creates connection**
   ```go
   conn, err := grpc.Dial("proglog:8080", ...)
   ```

2. **Resolver resolves name to addresses**
   ```
   proglog:8080 → [10.244.1.5:8080, 10.244.2.3:8080, ...]
   ```

3. **Picker selects address**
   ```
   round_robin → 10.244.1.5:8080
   ```

4. **Connection established**

### gRPC Resolver Interface

```go
type Resolver interface {
    // ResolveNow is called by gRPC to try to resolve the target name
    ResolveNow(ResolveNowOptions)
    
    // Close closes the resolver
    Close()
}

type Builder interface {
    // Build creates a new resolver for the given target
    Build(
        target Target,
        cc ClientConn,
        opts BuildOptions,
    ) (Resolver, error)
    
    // Scheme returns the scheme supported by this resolver
    Scheme() string
}
```

## Custom Resolver Implementation

### Resolver Builder

```go
const Name = "proglog"

type Resolver struct {
    mu            sync.Mutex
    clientConn    resolver.ClientConn
    resolverConn  *grpc.ClientConn
    serviceConfig *serviceconfig.ParseResult
    logger        *zap.Logger
}

var _ resolver.Builder = (*Resolver)(nil)

func (r *Resolver) Build(
    target resolver.Target,
    cc resolver.ClientConn,
    opts resolver.BuildOptions,
) (resolver.Resolver, error) {
    r.logger = zap.L().Named("resolver")
    r.clientConn = cc
    
    // Connect to service registry or use DNS
    conn, err := grpc.Dial(
        target.Endpoint,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return nil, err
    }
    r.resolverConn = conn
    
    // Start watching for updates
    go r.watch()
    
    return r, nil
}

func (r *Resolver) Scheme() string {
    return Name
}
```

### Watching for Updates

```go
func (r *Resolver) watch() {
    client := api.NewLogClient(r.resolverConn)
    
    // Get server list
    ctx := context.Background()
    stream, err := client.GetServers(ctx, &api.GetServersRequest{})
    if err != nil {
        r.logger.Error("failed to get servers", zap.Error(err))
        return
    }
    
    // Watch for server changes
    for {
        res, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            r.logger.Error("failed to receive server", zap.Error(err))
            return
        }
        
        r.resolve(res.Servers)
    }
}

func (r *Resolver) resolve(servers []*api.Server) {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // Convert to gRPC addresses
    var addrs []resolver.Address
    for _, server := range servers {
        addrs = append(addrs, resolver.Address{
            Addr: server.RpcAddr,
            Attributes: attributes.New(
                "is_leader", server.IsLeader,
            ),
        })
    }
    
    // Update client connection
    r.clientConn.UpdateState(resolver.State{
        Addresses:     addrs,
        ServiceConfig: r.serviceConfig,
    })
}
```

### Registering the Resolver

```go
func init() {
    resolver.Register(&Resolver{})
}
```

## Load Balancing

### Built-in Load Balancers

gRPC provides several built-in load balancers:

**Round Robin:**
```go
conn, err := grpc.Dial(
    "proglog:8080",
    grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
)
```

**Pick First:**
```go
conn, err := grpc.Dial(
    "proglog:8080",
    grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"pick_first"}`),
)
```

### Custom Picker Implementation

For more control, implement a custom picker:

```go
type Picker struct {
    mu        sync.RWMutex
    leader    resolver.Address
    followers []resolver.Address
}

func (p *Picker) Pick(info balancer.PickInfo) (
    balancer.PickResult,
    error,
) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    var addrs []resolver.Address
    
    // Route produce requests to leader
    if strings.Contains(info.FullMethodName, "Produce") {
        addrs = append(addrs, p.leader)
    } else {
        // Route consume requests to followers (for read scaling)
        addrs = p.followers
    }
    
    if len(addrs) == 0 {
        return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
    }
    
    // Pick randomly from available addresses
    addr := addrs[rand.Intn(len(addrs))]
    
    return balancer.PickResult{
        SubConn: p.subConns[addr],
    }, nil
}
```

### Picker Builder

```go
type PickerBuilder struct{}

func (p *PickerBuilder) Build(buildInfo base.PickerBuildInfo) balancer.Picker {
    picker := &Picker{
        subConns: make(map[resolver.Address]balancer.SubConn),
    }
    
    for addr, sc := range buildInfo.ReadySCs {
        isLeader := addr.Attributes.Value("is_leader").(bool)
        if isLeader {
            picker.leader = addr
            picker.subConns[addr] = sc
        } else {
            picker.followers = append(picker.followers, addr)
            picker.subConns[addr] = sc
        }
    }
    
    return picker
}
```

### Registering Custom Load Balancer

```go
const Name = "proglog"

func init() {
    balancer.Register(
        base.NewBalancerBuilder(
            Name,
            &PickerBuilder{},
            base.Config{HealthCheck: true},
        ),
    )
}
```

## Health Checking

### Server Health Check

```go
type HealthChecker struct {
    log *log.Log
}

func (h *HealthChecker) Check(
    ctx context.Context,
    req *grpc_health_v1.HealthCheckRequest,
) (*grpc_health_v1.HealthCheckResponse, error) {
    // Check log health
    if h.log.IsHealthy() {
        return &grpc_health_v1.HealthCheckResponse{
            Status: grpc_health_v1.HealthCheckResponse_SERVING,
        }, nil
    }
    
    return &grpc_health_v1.HealthCheckResponse{
        Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
    }, nil
}
```

### Client Health Checking

```go
conn, err := grpc.Dial(
    "proglog:8080",
    grpc.WithDefaultServiceConfig(`{
        "healthCheckConfig": {
            "serviceName": "proglog.Log"
        },
        "loadBalancingPolicy": "round_robin"
    }`),
)
```

gRPC automatically:
- Checks server health periodically
- Removes unhealthy servers
- Re-adds recovered servers

## Usage Example

### Client Configuration

```go
import _ "your-module/internal/discovery"  // Register resolver

conn, err := grpc.Dial(
    fmt.Sprintf(
        "%s:///%s",
        discovery.Name,  // "proglog"
        "localhost:8080",
    ),
    grpc.WithDefaultServiceConfig(fmt.Sprintf(`{
        "loadBalancingConfig": [{"%s":{}}]
    }`, loadbalance.Name)),
    grpc.WithTransportCredentials(insecure.NewCredentials()),
)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

client := api.NewLogClient(conn)
```

### Automatic Load Balancing

```go
// All requests automatically load balanced
for i := 0; i < 100; i++ {
    // Each request goes to a different server (round-robin)
    res, err := client.Produce(ctx, &api.ProduceRequest{
        Record: &api.Record{
            Value: []byte(fmt.Sprintf("message %d", i)),
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Offset: %d\n", res.Offset)
}
```

## Testing Load Balancing

### Test Setup

```go
func testServerSideLoadBalancing(t *testing.T) {
    // Start multiple servers
    servers := []*grpc.Server{}
    addrs := []string{}
    
    for i := 0; i < 3; i++ {
        l, err := net.Listen("tcp", "127.0.0.1:0")
        require.NoError(t, err)
        
        server, err := NewGRPCServer(&Config{...})
        require.NoError(t, err)
        
        servers = append(servers, server)
        addrs = append(addrs, l.Addr().String())
        
        go server.Serve(l)
    }
    
    // Client connects using resolver
    conn, err := grpc.Dial(
        fmt.Sprintf("%s:///%s", resolver.Name, addrs[0]),
        grpc.WithDefaultServiceConfig(fmt.Sprintf(`{
            "loadBalancingConfig": [{"%s":{}}]
        }`, loadbalance.Name)),
    )
    require.NoError(t, err)
    
    client := api.NewLogClient(conn)
    
    // Verify requests distributed across servers
    // ... test logic ...
}
```

## Load Balancing Strategies

### 1. Round Robin
```
Request 1 → Server A
Request 2 → Server B
Request 3 → Server C
Request 4 → Server A
```
**Use when:** Servers have equal capacity

### 2. Least Connections
```
Server A: 10 connections
Server B: 5 connections  ← Route here
Server C: 8 connections
```
**Use when:** Request processing time varies

### 3. Weighted Round Robin
```
Server A (weight=2): 40% traffic
Server B (weight=2): 40% traffic
Server C (weight=1): 20% traffic
```
**Use when:** Servers have different capacities

### 4. Consistent Hashing
```
hash(request) → Server
```
**Use when:** Need request affinity (same user → same server)

## Advanced Features

### Connection Pooling

```go
// gRPC automatically pools connections
conn, err := grpc.Dial(
    addr,
    grpc.WithDefaultServiceConfig(`{
        "methodConfig": [{
            "name": [{"service": "proglog.Log"}],
            "waitForReady": true,
            "retryPolicy": {
                "maxAttempts": 5,
                "initialBackoff": "0.5s",
                "maxBackoff": "30s",
                "backoffMultiplier": 2,
                "retryableStatusCodes": ["UNAVAILABLE"]
            }
        }]
    }`),
)
```

### Circuit Breaking

```go
type CircuitBreaker struct {
    failures  int
    threshold int
    timeout   time.Duration
    state     State  // CLOSED, OPEN, HALF_OPEN
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == OPEN {
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = HALF_OPEN
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        if cb.failures >= cb.threshold {
            cb.state = OPEN
        }
        return err
    }
    
    cb.failures = 0
    cb.state = CLOSED
    return nil
}
```

## Best Practices

### Resolver
- ✅ Cache resolved addresses
- ✅ Handle resolver failures gracefully
- ✅ Implement exponential backoff for retries
- ✅ Log resolution changes
- ✅ Support dynamic updates

### Load Balancing
- ✅ Monitor server health
- ✅ Consider server capacity
- ✅ Implement gradual rollout
- ✅ Use sticky sessions when needed
- ✅ Test failover scenarios

### Health Checks
- ✅ Check application health, not just TCP
- ✅ Use appropriate intervals (5-10s)
- ✅ Implement graceful shutdown
- ✅ Mark unhealthy before shutdown
- ✅ Monitor health check failures

## Key Takeaways

✅ Server-side discovery abstracts routing from clients  
✅ DNS provides simple, standard service discovery  
✅ Custom resolvers enable advanced routing logic  
✅ Load balancing distributes traffic efficiently  
✅ Health checking ensures high availability  

## Next Steps

Move to **Chapter 8: Client-Side Service Discovery** to learn how to:
- Implement client-side discovery with Serf
- Use gossip protocols for cluster membership
- Handle dynamic cluster changes
- Compare client-side vs server-side patterns

## Dependencies

```
google.golang.org/grpc v1.50.1               # gRPC framework
google.golang.org/grpc/health v1.50.1        # Health checking
```

## Additional Resources

- [gRPC Load Balancing](https://grpc.io/blog/grpc-load-balancing/)
- [gRPC Name Resolution](https://github.com/grpc/grpc/blob/master/doc/naming.md)
- [Service Discovery Patterns](https://microservices.io/patterns/service-registry.html)

---

**Ready for client-side discovery?** The next chapter implements gossip-based discovery with Serf! 💬