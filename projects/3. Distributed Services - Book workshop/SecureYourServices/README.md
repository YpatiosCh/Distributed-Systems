# Chapter 5: Secure Your Services

## Overview

This chapter adds enterprise-grade security to your distributed log service. You'll implement TLS encryption for data in transit, mutual TLS (mTLS) for authentication, and access control lists (ACLs) for fine-grained authorization.

## What You'll Learn

- Generating and managing TLS certificates
- Implementing mutual TLS authentication
- Building authorization with Casbin ACLs
- Securing gRPC services
- Certificate-based identity management
- Testing secure services

## Security Fundamentals

### The Security Triad

1. **Encryption** (Confidentiality)
   - TLS encrypts data in transit
   - Prevents eavesdropping
   - Protects sensitive data

2. **Authentication** (Identity)
   - Mutual TLS verifies both parties
   - Client certificates prove identity
   - Prevents impersonation

3. **Authorization** (Access Control)
   - ACLs define who can do what
   - Fine-grained permissions
   - Principle of least privilege

## Project Structure

```
SecureServices/
├── api/
│   └── v1/
│       ├── log.proto
│       ├── log.pb.go
│       ├── log_grpc.pb.go
│       └── error.go          # Custom error types
├── internal/
│   ├── auth/
│   │   ├── authorizer.go     # ACL authorization
│   │   └── authorizer_test.go
│   ├── config/
│   │   ├── files.go          # Certificate paths
│   │   └── tls.go            # TLS configuration
│   ├── log/
│   │   └── ...               # Log implementation
│   └── server/
│       ├── server.go         # Secured gRPC server
│       └── server_test.go
├── test/
│   ├── ca.pem                # Certificate Authority
│   ├── server.pem            # Server certificate
│   ├── server-key.pem        # Server private key
│   ├── client.pem            # Client certificate
│   ├── client-key.pem        # Client private key
│   ├── model.conf            # Casbin model
│   └── policy.csv            # Casbin policies
├── go.mod
└── go.sum
```

## TLS Certificates

### Certificate Hierarchy

```
Root CA (Certificate Authority)
    ├── Server Certificate
    │   └── Used by gRPC server
    └── Client Certificates
        ├── root-client (full access)
        └── nobody-client (limited access)
```

### Generating Certificates

Use CloudFlare's `cfssl` tool:

```bash
# Install cfssl
go install github.com/cloudflare/cfssl/cmd/cfssl@latest
go install github.com/cloudflare/cfssl/cmd/cfssljson@latest

# Generate CA
cfssl gencert -initca ca-csr.json | cfssljson -bare ca

# Generate Server Certificate
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=ca-config.json \
  -profile=server \
  server-csr.json | cfssljson -bare server

# Generate Client Certificate
cfssl gencert \
  -ca=ca.pem \
  -ca-key=ca-key.pem \
  -config=ca-config.json \
  -profile=client \
  client-csr.json | cfssljson -bare client
```

### Certificate Configuration Files

**ca-csr.json** (Certificate Authority):
```json
{
  "CN": "My Log Service CA",
  "key": {
    "algo": "rsa",
    "size": 2048
  },
  "names": [{
    "C": "US",
    "ST": "CA",
    "L": "San Francisco"
  }]
}
```

**server-csr.json** (Server):
```json
{
  "CN": "127.0.0.1",
  "hosts": [
    "127.0.0.1",
    "localhost"
  ],
  "key": {
    "algo": "rsa",
    "size": 2048
  }
}
```

## TLS Configuration

### Server TLS Setup

```go
func SetupTLSConfig(cfg TLSConfig) (*tls.Config, error) {
    var err error
    tlsConfig := &tls.Config{}
    
    if cfg.CertFile != "" && cfg.KeyFile != "" {
        // Load server certificate
        tlsConfig.Certificates = make([]tls.Certificate, 1)
        tlsConfig.Certificates[0], err = tls.LoadX509KeyPair(
            cfg.CertFile,
            cfg.KeyFile,
        )
        if err != nil {
            return nil, err
        }
    }
    
    if cfg.CAFile != "" {
        // Load CA for client verification
        b, err := ioutil.ReadFile(cfg.CAFile)
        if err != nil {
            return nil, err
        }
        ca := x509.NewCertPool()
        ok := ca.AppendCertsFromPEM(b)
        if !ok {
            return nil, fmt.Errorf("failed to parse CA certificate")
        }
        
        if cfg.Server {
            tlsConfig.ClientCAs = ca
            tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
        } else {
            tlsConfig.RootCAs = ca
        }
    }
    
    return tlsConfig, nil
}
```

### Applying TLS to gRPC Server

```go
func NewGRPCServer(config *Config, opts ...grpc.ServerOption) (
    *grpc.Server,
    error,
) {
    // Setup TLS credentials
    serverTLSConfig, err := config.ServerTLSConfig, nil
    if err != nil {
        return nil, err
    }
    creds := credentials.NewTLS(serverTLSConfig)
    
    opts = append(opts, grpc.Creds(creds))
    
    gsrv := grpc.NewServer(opts...)
    srv := &grpcServer{
        Config: config,
    }
    api.RegisterLogServer(gsrv, srv)
    
    return gsrv, nil
}
```

### Client TLS Configuration

```go
clientTLSConfig, err := config.SetupTLSConfig(config.ClientTLSConfig)
if err != nil {
    return nil, err
}
tlsCreds := credentials.NewTLS(clientTLSConfig)
opts := []grpc.DialOption{grpc.WithTransportCredentials(tlsCreds)}

conn, err := grpc.Dial(addr, opts...)
if err != nil {
    return nil, err
}

client := api.NewLogClient(conn)
```

## Authentication with Client Certificates

### Extracting Client Identity

```go
func subject(ctx context.Context) string {
    p, ok := peer.FromContext(ctx)
    if !ok {
        return ""
    }
    
    tlsInfo := p.AuthInfo.(credentials.TLSInfo)
    if len(tlsInfo.State.VerifiedChains) == 0 {
        return ""
    }
    
    return tlsInfo.State.VerifiedChains[0][0].Subject.CommonName
}
```

This extracts the Common Name (CN) from the client's certificate, which we use as the client's identity.

## Authorization with Casbin ACLs

### Casbin Model (model.conf)

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
```

**Components:**
- `sub` (subject) - Who (client CN)
- `obj` (object) - What (resource)
- `act` (action) - Action (produce, consume)

### Policy File (policy.csv)

```csv
p, root, *, produce
p, root, *, consume
p, nobody, *, consume
```

**Rules:**
- `root` can produce and consume
- `nobody` can only consume

### Authorizer Implementation

```go
type Authorizer struct {
    enforcer *casbin.Enforcer
}

func New(model, policy string) (*Authorizer, error) {
    enforcer, err := casbin.NewEnforcer(model, policy)
    if err != nil {
        return nil, err
    }
    return &Authorizer{
        enforcer: enforcer,
    }, nil
}

func (a *Authorizer) Authorize(subject, object, action string) error {
    if !a.enforcer.Enforce(subject, object, action) {
        msg := fmt.Sprintf(
            "%s not permitted to %s to %s",
            subject,
            action,
            object,
        )
        return status.New(codes.PermissionDenied, msg).Err()
    }
    return nil
}
```

### Applying Authorization in RPCs

```go
func (s *grpcServer) Produce(
    ctx context.Context,
    req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
    // Authenticate: extract identity
    subject := subject(ctx)
    
    // Authorize: check permissions
    if err := s.Authorizer.Authorize(
        subject,
        "produce",
        api.Produce,
    ); err != nil {
        return nil, err
    }
    
    // Execute if authorized
    offset, err := s.CommitLog.Append(req.Record)
    if err != nil {
        return nil, err
    }
    return &api.ProduceResponse{Offset: offset}, nil
}
```

## gRPC Interceptors

For cleaner code, use interceptors to apply authorization globally:

```go
func authenticate(ctx context.Context) (context.Context, error) {
    peer, ok := peer.FromContext(ctx)
    if !ok {
        return ctx, status.New(
            codes.Unknown,
            "couldn't find peer info",
        ).Err()
    }
    
    if peer.AuthInfo == nil {
        return context.WithValue(ctx, subjectContextKey{}, ""), nil
    }
    
    tlsInfo := peer.AuthInfo.(credentials.TLSInfo)
    subject := tlsInfo.State.VerifiedChains[0][0].Subject.CommonName
    ctx = context.WithValue(ctx, subjectContextKey{}, subject)
    
    return ctx, nil
}
```

Apply as an interceptor:

```go
opts := []grpc.ServerOption{
    grpc.UnaryInterceptor(
        grpc_middleware.ChainUnaryServer(
            grpc_auth.UnaryServerInterceptor(authenticate),
        ),
    ),
    grpc.StreamInterceptor(
        grpc_middleware.ChainStreamServer(
            grpc_auth.StreamServerInterceptor(authenticate),
        ),
    ),
}
```

## Testing Secured Services

### Test Setup with Different Clients

```go
func testProduceConsume(t *testing.T) {
    // Setup server
    serverTLSConfig, err := config.SetupTLSConfig(config.TLSConfig{
        CertFile:      config.ServerCertFile,
        KeyFile:       config.ServerKeyFile,
        CAFile:        config.CAFile,
        Server:        true,
    })
    require.NoError(t, err)
    
    // Test with root client (should succeed)
    rootClient := newClient(t, config.RootClientCertFile, config.RootClientKeyFile)
    _, err = rootClient.Produce(ctx, &api.ProduceRequest{
        Record: &api.Record{Value: []byte("hello world")},
    })
    require.NoError(t, err)
    
    // Test with nobody client (should fail produce)
    nobodyClient := newClient(t, config.NobodyClientCertFile, config.NobodyClientKeyFile)
    _, err = nobodyClient.Produce(ctx, &api.ProduceRequest{
        Record: &api.Record{Value: []byte("hello world")},
    })
    require.Error(t, err)
    gotCode := status.Code(err)
    wantCode := codes.PermissionDenied
    require.Equal(t, wantCode, gotCode)
}
```

## Security Best Practices

### Certificate Management
- ✅ Use short-lived certificates
- ✅ Rotate certificates regularly
- ✅ Store private keys securely
- ✅ Use strong key algorithms (RSA 2048+, ECDSA)
- ✅ Validate certificates on both sides

### Authorization
- ✅ Principle of least privilege
- ✅ Explicit deny over implicit allow
- ✅ Audit all authorization decisions
- ✅ Review policies regularly
- ✅ Use role-based access control (RBAC)

### TLS Configuration
- ✅ Require TLS 1.2 or higher
- ✅ Use strong cipher suites
- ✅ Enable perfect forward secrecy
- ✅ Verify hostname/IP in certificates
- ✅ Implement certificate pinning for critical services

## Common Security Pitfalls

❌ **Self-signed certificates in production** - Use proper CA  
❌ **Skipping certificate validation** - Always verify  
❌ **Hardcoded credentials** - Use environment variables or secrets manager  
❌ **Overly permissive ACLs** - Start restrictive, open as needed  
❌ **No audit logging** - Track all authorization decisions  

## Error Handling

### Custom Error Types

```go
var (
    ErrOffsetOutOfRange = &errOffsetOutOfRange{}
)

type errOffsetOutOfRange struct{}

func (e errOffsetOutOfRange) Error() string {
    return "offset out of range"
}

func (e errOffsetOutOfRange) GRPCStatus() *status.Status {
    return status.New(codes.OutOfRange, e.Error())
}
```

## Key Takeaways

✅ TLS encrypts data in transit  
✅ Mutual TLS authenticates both client and server  
✅ ACLs provide fine-grained authorization  
✅ Certificate-based identity is more secure than passwords  
✅ Interceptors enable clean, reusable security logic  

## Next Steps

Move to **Chapter 6: Observe Your Systems** to learn how to:
- Add structured logging
- Implement metrics with Prometheus
- Enable distributed tracing
- Monitor service health

## Dependencies

```
github.com/casbin/casbin v1.9.1             # ACL authorization
github.com/grpc-ecosystem/go-grpc-middleware # Interceptor utilities
google.golang.org/grpc v1.50.1               # gRPC framework
```

## Additional Resources

- [TLS Best Practices](https://github.com/ssllabs/research/wiki/SSL-and-TLS-Deployment-Best-Practices)
- [gRPC Authentication Guide](https://grpc.io/docs/guides/auth/)
- [Casbin Documentation](https://casbin.org/docs/overview)
- [OWASP Transport Layer Protection](https://cheatsheetseries.owasp.org/cheatsheets/Transport_Layer_Protection_Cheat_Sheet.html)

---

**Ready to monitor your service?** The next chapter adds observability with logging, metrics, and tracing! 📊