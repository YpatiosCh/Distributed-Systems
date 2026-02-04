# Chapter 6: Observe Your Systems

## Overview

This chapter implements comprehensive observability for your distributed log service using structured logging, metrics collection, and distributed tracing. You'll learn industry-standard patterns for monitoring production services.

## What You'll Learn

- Implementing structured logging with Uber's Zap
- Collecting metrics with OpenTelemetry and Prometheus
- Distributed tracing across service calls
- Exporting telemetry data
- Debugging production issues with observability
- Understanding the three pillars of observability

## The Three Pillars of Observability

### 1. **Logs** (Events)
What happened and when?
- Discrete events
- Detailed context
- Debug information
- Error messages

### 2. **Metrics** (Aggregations)
How is the system performing?
- Counters, gauges, histograms
- Aggregated over time
- Efficient storage
- Alerting triggers

### 3. **Traces** (Request Flow)
Where is time being spent?
- Request path through services
- Latency breakdown
- Dependencies
- Bottleneck identification

## Project Structure

```
ObserveYourSystems/
├── api/
│   └── v1/
│       └── ...
├── internal/
│   ├── log/
│   │   └── ...
│   └── server/
│       ├── server.go         # Instrumented gRPC server
│       ├── log.go
│       └── server_test.go
├── go.mod
└── go.sum
```

## Structured Logging with Zap

### Why Structured Logging?

**Traditional Logging:**
```go
log.Printf("User %s logged in from %s at %s", user, ip, time)
```

**Problems:**
- Hard to parse
- No type safety
- Difficult to query
- Can't aggregate

**Structured Logging:**
```go
logger.Info("user logged in",
    zap.String("user", user),
    zap.String("ip", ip),
    zap.Time("timestamp", time),
)
```

**Benefits:**
- ✅ Machine-readable (JSON)
- ✅ Type-safe
- ✅ Easy to query/aggregate
- ✅ Consistent format

### Setting Up Zap

```go
import "go.uber.org/zap"

// Development logging (human-readable)
logger, _ := zap.NewDevelopment()
defer logger.Sync()

// Production logging (JSON)
logger, _ := zap.NewProduction()
defer logger.Sync()

// Custom configuration
config := zap.NewProductionConfig()
config.OutputPaths = []string{"stdout", "/var/log/app.log"}
logger, _ := config.Build()
```

### Logging in gRPC Handlers

```go
func (s *grpcServer) Produce(
    ctx context.Context,
    req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
    offset, err := s.CommitLog.Append(req.Record)
    if err != nil {
        s.logger.Error("failed to append record",
            zap.Error(err),
            zap.Binary("record.value", req.Record.Value),
        )
        return nil, err
    }
    
    s.logger.Info("record appended",
        zap.Uint64("offset", offset),
        zap.Int("size", len(req.Record.Value)),
    )
    
    return &api.ProduceResponse{Offset: offset}, nil
}
```

### Contextual Logging

Add context to logger for related operations:

```go
// Add request ID to logger
requestLogger := logger.With(
    zap.String("request_id", requestID),
    zap.String("method", "Produce"),
)

// All logs include request_id
requestLogger.Info("processing request")
requestLogger.Error("request failed", zap.Error(err))
```

## Metrics with OpenTelemetry

### Key Metric Types

1. **Counter** - Monotonically increasing
   - Total requests
   - Bytes written
   - Errors count

2. **Gauge** - Point-in-time value
   - Active connections
   - Queue size
   - Memory usage

3. **Histogram** - Distribution of values
   - Request latency
   - Response size
   - Processing time

### Setting Up OpenTelemetry Metrics

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

type Telemetry struct {
    meter metric.Meter
    
    // Counters
    produceTotal metric.Int64Counter
    consumeTotal metric.Int64Counter
    errorsTotal  metric.Int64Counter
    
    // Histograms
    produceLatency metric.Float64Histogram
    recordSize     metric.Int64Histogram
}

func NewTelemetry() (*Telemetry, error) {
    meter := otel.Meter("proglog")
    
    produceTotal, err := meter.Int64Counter(
        "log.produce.total",
        metric.WithDescription("Total produce requests"),
    )
    if err != nil {
        return nil, err
    }
    
    produceLatency, err := meter.Float64Histogram(
        "log.produce.latency",
        metric.WithDescription("Produce request latency"),
        metric.WithUnit("ms"),
    )
    if err != nil {
        return nil, err
    }
    
    return &Telemetry{
        meter:          meter,
        produceTotal:   produceTotal,
        produceLatency: produceLatency,
    }, nil
}
```

### Instrumenting Code with Metrics

```go
func (s *grpcServer) Produce(
    ctx context.Context,
    req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
    start := time.Now()
    
    // Increment request counter
    s.telemetry.produceTotal.Add(ctx, 1)
    
    offset, err := s.CommitLog.Append(req.Record)
    if err != nil {
        s.telemetry.errorsTotal.Add(ctx, 1)
        return nil, err
    }
    
    // Record latency
    duration := time.Since(start).Milliseconds()
    s.telemetry.produceLatency.Record(ctx, float64(duration))
    
    // Record size
    s.telemetry.recordSize.Record(ctx, int64(len(req.Record.Value)))
    
    return &api.ProduceResponse{Offset: offset}, nil
}
```

### Prometheus Exporter

```go
import (
    "go.opentelemetry.io/otel/exporters/prometheus"
    "go.opentelemetry.io/otel/sdk/metric"
)

// Setup Prometheus exporter
exporter, err := prometheus.New()
if err != nil {
    log.Fatal(err)
}

provider := metric.NewMeterProvider(
    metric.WithReader(exporter),
)
otel.SetMeterProvider(provider)

// Expose metrics endpoint
http.Handle("/metrics", promhttp.Handler())
go http.ListenAndServe(":9090", nil)
```

### Example Prometheus Queries

```promql
# Request rate (requests per second)
rate(log_produce_total[5m])

# Error rate percentage
rate(log_errors_total[5m]) / rate(log_produce_total[5m]) * 100

# 95th percentile latency
histogram_quantile(0.95, log_produce_latency_bucket)

# Average record size
avg(log_record_size)
```

## Distributed Tracing

### Why Tracing?

In distributed systems, a single user request may:
- Hit multiple services
- Trigger async operations
- Spawn background jobs
- Cross network boundaries

Tracing shows the complete path and timing.

### OpenTelemetry Tracing Setup

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
    "go.opentelemetry.io/otel/exporters/jaeger"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Initialize tracer
func initTracer() (*sdktrace.TracerProvider, error) {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint("http://localhost:14268/api/traces"),
        ),
    )
    if err != nil {
        return nil, err
    }
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("proglog"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

### Instrumenting RPCs with Tracing

```go
func (s *grpcServer) Produce(
    ctx context.Context,
    req *api.ProduceRequest,
) (*api.ProduceResponse, error) {
    tracer := otel.Tracer("proglog")
    
    // Start span
    ctx, span := tracer.Start(ctx, "Produce")
    defer span.End()
    
    // Add attributes
    span.SetAttributes(
        attribute.Int("record.size", len(req.Record.Value)),
    )
    
    // Create child span for append operation
    appendCtx, appendSpan := tracer.Start(ctx, "log.Append")
    offset, err := s.CommitLog.Append(req.Record)
    appendSpan.End()
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }
    
    span.SetAttributes(attribute.Int64("offset", int64(offset)))
    return &api.ProduceResponse{Offset: offset}, nil
}
```

### gRPC Tracing Interceptors

```go
import (
    "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
)

// Server-side
opts := []grpc.ServerOption{
    grpc.UnaryInterceptor(
        otelgrpc.UnaryServerInterceptor(),
    ),
    grpc.StreamInterceptor(
        otelgrpc.StreamServerInterceptor(),
    ),
}
server := grpc.NewServer(opts...)

// Client-side
conn, err := grpc.Dial(
    addr,
    grpc.WithUnaryInterceptor(
        otelgrpc.UnaryClientInterceptor(),
    ),
    grpc.WithStreamInterceptor(
        otelgrpc.StreamClientInterceptor(),
    ),
)
```

## Visualization Tools

### Prometheus + Grafana (Metrics)

**Prometheus** - Metrics collection and storage
- Scrapes `/metrics` endpoint
- Time-series database
- PromQL query language
- Alerting rules

**Grafana** - Metrics visualization
- Beautiful dashboards
- Multiple data sources
- Alerting integration
- Templating

### Jaeger (Traces)

- Trace collection and storage
- Service dependency graph
- Latency analysis
- Error tracking

**Example Trace:**
```
Request → API Gateway (5ms)
  ├→ Log Service (45ms)
  │  ├→ Append (30ms)
  │  │  ├→ Write Store (20ms)
  │  │  └→ Write Index (10ms)
  │  └→ Replicate (15ms)
  └→ Total: 50ms
```

## Best Practices

### Logging
- ✅ Use structured logging
- ✅ Log at appropriate levels (DEBUG, INFO, WARN, ERROR)
- ✅ Include context (request ID, user ID)
- ✅ Don't log sensitive data (passwords, tokens)
- ✅ Log errors with stack traces
- ❌ Don't log in hot paths (high-frequency loops)

### Metrics
- ✅ Use descriptive names: `log_produce_requests_total`
- ✅ Include units in description
- ✅ Use labels sparingly (cardinality)
- ✅ Track both successes and failures
- ✅ Monitor SLIs (latency, error rate, throughput)
- ❌ Don't use high-cardinality labels (user IDs)

### Tracing
- ✅ Trace critical paths
- ✅ Add meaningful span names
- ✅ Include relevant attributes
- ✅ Sample appropriately (not 100% in prod)
- ✅ Propagate context across boundaries
- ❌ Don't create too many spans (overhead)

## Sampling Strategies

For high-traffic services, sample traces:

```go
// Sample 10% of traces
sampler := sdktrace.ParentBased(
    sdktrace.TraceIDRatioBased(0.1),
)

tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sampler),
)
```

**Strategies:**
- Always sample errors
- Sample head (first request)
- Sample tail (based on latency)
- Adaptive sampling (adjust based on traffic)

## Alerting

### Key Alerts

1. **Error Rate** - `> 1%`
2. **Latency** - `p95 > 100ms`
3. **Availability** - `< 99.9%`
4. **Disk Usage** - `> 80%`
5. **Memory** - `> 85%`

### Prometheus Alert Example

```yaml
groups:
  - name: log_service
    rules:
      - alert: HighErrorRate
        expr: rate(log_errors_total[5m]) / rate(log_produce_total[5m]) > 0.01
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"
```

## Key Takeaways

✅ Observability is critical for production systems  
✅ Structured logging enables analysis  
✅ Metrics provide system health visibility  
✅ Tracing reveals performance bottlenecks  
✅ The three pillars work together for complete visibility  

## Next Steps

Move to **Chapter 7: Server-Side Service Discovery** to learn how to:
- Discover services automatically
- Route requests to healthy servers
- Load balance across instances
- Handle server failures gracefully

## Dependencies

```
go.uber.org/zap v1.24.0                                           # Structured logging
go.opentelemetry.io/otel v1.11.1                                 # Observability framework
go.opentelemetry.io/otel/exporters/prometheus v0.34.0            # Prometheus exporter
go.opentelemetry.io/otel/exporters/jaeger v1.11.1                # Jaeger exporter
go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc # gRPC instrumentation
```

## Additional Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [The USE Method](http://www.brendangregg.com/usemethod.html)
- [The RED Method](https://grafana.com/blog/2018/08/02/the-red-method-how-to-instrument-your-services/)

---

**Ready for service discovery?** The next chapter makes your services discoverable and resilient! 🔍