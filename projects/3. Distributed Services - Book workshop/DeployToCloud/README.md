# Chapter 10: Deploy Applications to the Cloud

## Overview

This chapter takes your distributed log service to production by deploying it on Kubernetes. You'll learn how to containerize your service, use StatefulSets for persistent storage, configure Helm charts, and implement cloud-native patterns.

## What You'll Learn

- Containerizing Go applications with Docker
- Kubernetes fundamentals (Pods, Services, StatefulSets)
- Managing state in Kubernetes
- Helm chart creation and deployment
- Service discovery in Kubernetes
- Production-ready configurations
- Cloud deployment best practices

## Cloud-Native Architecture

### From Development to Production

**Development:**
- Single machine
- Local storage
- Manual setup
- No redundancy

**Production:**
- Multiple nodes
- Distributed storage
- Automated deployment
- High availability

## Project Structure

```
DeployToCloud/
├── cmd/
│   └── proglog/
│       └── main.go           # Application entrypoint
├── deploy/
│   ├── proglog/              # Helm chart
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   ├── templates/
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   └── configmap.yaml
│   │   └── hooks/
│   │       └── delete-service-per-pod.jsonnet
│   └── metacontroller/       # Custom controller
│       ├── Chart.yaml
│       └── templates/
│           └── ...
├── Dockerfile
├── go.mod
└── go.sum
```

## Docker Container

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.23-alpine AS build
WORKDIR /go/src/proglog
COPY . .
RUN CGO_ENABLED=0 go build -o /go/bin/proglog ./cmd/proglog

# Runtime stage
FROM scratch
COPY --from=build /go/bin/proglog /bin/proglog
ENTRYPOINT ["/bin/proglog"]
```

**Multi-stage build benefits:**
- ✅ Smaller final image (only binary, no Go toolchain)
- ✅ Faster builds (cached layers)
- ✅ More secure (minimal attack surface)
- ✅ FROM scratch = <10MB images

### Building the Image

```bash
# Build
docker build -t proglog:latest .

# Tag for registry
docker tag proglog:latest registry.example.com/proglog:v1.0.0

# Push to registry
docker push registry.example.com/proglog:v1.0.0

# Run locally
docker run -p 8080:8080 proglog:latest \
  --config-file=/etc/proglog/config.yaml
```

## Kubernetes Resources

### Pod

A Pod is the smallest deployable unit:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: proglog
spec:
  containers:
  - name: proglog
    image: proglog:latest
    ports:
    - containerPort: 8080
      name: rpc
    - containerPort: 8401
      name: serf
```

### Service

A Service provides stable networking:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: proglog
spec:
  selector:
    app: proglog
  ports:
  - port: 8080
    targetPort: 8080
    name: rpc
  - port: 8401
    targetPort: 8401
    name: serf
  clusterIP: None  # Headless service for StatefulSet
```

**Headless Service:**
- No cluster IP
- DNS returns individual pod IPs
- Enables direct pod-to-pod communication
- Required for StatefulSets

### StatefulSet

StatefulSets provide:
- Stable network identities
- Persistent storage
- Ordered deployment/scaling

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: proglog
spec:
  serviceName: proglog
  replicas: 3
  selector:
    matchLabels:
      app: proglog
  template:
    metadata:
      labels:
        app: proglog
    spec:
      containers:
      - name: proglog
        image: proglog:latest
        ports:
        - containerPort: 8080
          name: rpc
        - containerPort: 8401
          name: serf
        args:
          - --config-file=/etc/proglog/config.yaml
        volumeMounts:
        - name: data
          mountPath: /var/run/proglog
        - name: config
          mountPath: /etc/proglog
          readOnly: true
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: 1Gi
```

**Pod Naming:**
```
proglog-0  # Stable identity
proglog-1
proglog-2
```

**DNS Names:**
```
proglog-0.proglog.default.svc.cluster.local
proglog-1.proglog.default.svc.cluster.local
proglog-2.proglog.default.svc.cluster.local
```

### ConfigMap

Store configuration separately from code:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: proglog-config
data:
  config.yaml: |
    data-dir: /var/run/proglog/data
    rpc-port: 8080
    bind-addr: $(POD_IP):8401
    bootstrap: false
    start-join-addrs:
      - proglog-0.proglog.default.svc.cluster.local:8401
    server-tls-cert-file: /etc/proglog/server.pem
    server-tls-key-file: /etc/proglog/server-key.pem
    server-tls-ca-file: /etc/proglog/ca.pem
    peer-tls-cert-file: /etc/proglog/peer.pem
    peer-tls-key-file: /etc/proglog/peer-key.pem
    peer-tls-ca-file: /etc/proglog/ca.pem
```

### Secrets

Store sensitive data:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: proglog-tls
type: Opaque
data:
  ca.pem: <base64-encoded>
  server.pem: <base64-encoded>
  server-key.pem: <base64-encoded>
  peer.pem: <base64-encoded>
  peer-key.pem: <base64-encoded>
```

Mount in StatefulSet:

```yaml
volumeMounts:
- name: tls
  mountPath: /etc/proglog
  readOnly: true
volumes:
- name: tls
  secret:
    secretName: proglog-tls
```

## Helm Chart

Helm packages Kubernetes resources:

### Chart.yaml

```yaml
apiVersion: v2
name: proglog
description: A distributed commit log
version: 0.1.0
appVersion: 1.0.0
```

### values.yaml

```yaml
replicaCount: 3

image:
  repository: proglog
  tag: latest
  pullPolicy: IfNotPresent

storage:
  size: 1Gi
  storageClass: standard

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

service:
  rpcPort: 8080
  serfPort: 8401
```

### Template: StatefulSet

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: {{ include "proglog.fullname" . }}
spec:
  serviceName: {{ include "proglog.fullname" . }}
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      {{- include "proglog.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "proglog.selectorLabels" . | nindent 8 }}
    spec:
      containers:
      - name: proglog
        image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
        imagePullPolicy: {{ .Values.image.pullPolicy }}
        ports:
        - containerPort: {{ .Values.service.rpcPort }}
          name: rpc
        - containerPort: {{ .Values.service.serfPort }}
          name: serf
        resources:
          {{- toYaml .Values.resources | nindent 12 }}
        volumeMounts:
        - name: data
          mountPath: /var/run/proglog
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: [ "ReadWriteOnce" ]
      resources:
        requests:
          storage: {{ .Values.storage.size }}
      storageClassName: {{ .Values.storage.storageClass }}
```

### Installing the Chart

```bash
# Install
helm install proglog ./deploy/proglog

# Upgrade
helm upgrade proglog ./deploy/proglog

# Uninstall
helm uninstall proglog

# Install with custom values
helm install proglog ./deploy/proglog \
  --set replicaCount=5 \
  --set storage.size=10Gi
```

## Bootstrap Process

### First Node (proglog-0)

```yaml
env:
- name: BOOTSTRAP
  value: "true"
```

proglog-0 bootstraps the Raft cluster.

### Subsequent Nodes

```yaml
env:
- name: BOOTSTRAP
  value: "false"
- name: START_JOIN_ADDRS
  value: "proglog-0.proglog.default.svc.cluster.local:8401"
```

Other nodes join the cluster through proglog-0.

## Service Per Pod Pattern

For advanced routing, create a service per pod using Metacontroller:

### Service Per Pod CompositeController

```yaml
apiVersion: metacontroller.k8s.io/v1alpha1
kind: CompositeController
metadata:
  name: service-per-pod
spec:
  generateSelector: true
  parentResource:
    apiVersion: apps/v1
    resource: statefulsets
  childResources:
  - apiVersion: v1
    resource: services
  hooks:
    sync:
      webhook:
        url: http://service-per-pod.metacontroller/sync
```

This creates:
```
proglog-0-service → proglog-0
proglog-1-service → proglog-1
proglog-2-service → proglog-2
```

## Monitoring

### Prometheus

Scrape metrics endpoint:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: proglog
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9090"
    prometheus.io/path: "/metrics"
spec:
  # ... service config
```

### Liveness Probe

Check if container needs restart:

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
```

### Readiness Probe

Check if pod ready for traffic:

```yaml
readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
```

## Scaling

### Manual Scaling

```bash
kubectl scale statefulset proglog --replicas=5
```

### Horizontal Pod Autoscaler

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: proglog
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: StatefulSet
    name: proglog
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

## Networking

### Ingress

External access:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: proglog
spec:
  rules:
  - host: proglog.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: proglog
            port:
              number: 8080
```

### Network Policy

Restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: proglog
spec:
  podSelector:
    matchLabels:
      app: proglog
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: client
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: proglog
```

## Storage

### Persistent Volume Claim

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: proglog-data
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: fast-ssd
```

### Storage Classes

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/aws-ebs
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
```

## Production Best Practices

### Resource Management

```yaml
resources:
  requests:
    cpu: 100m       # Guaranteed
    memory: 128Mi
  limits:
    cpu: 500m       # Max
    memory: 512Mi
```

### Pod Disruption Budget

Prevent too many pods down:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: proglog
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: proglog
```

### Anti-Affinity

Spread pods across nodes:

```yaml
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
    - labelSelector:
        matchLabels:
          app: proglog
      topologyKey: kubernetes.io/hostname
```

### Security Context

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  capabilities:
    drop:
    - ALL
  readOnlyRootFilesystem: true
```

## Deployment Strategies

### Rolling Update

Default strategy:

```yaml
spec:
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
```

### Blue-Green Deployment

Two identical environments:
- Blue: Current version
- Green: New version
- Switch traffic when ready

### Canary Deployment

Gradual rollout:
- 10% → New version
- Monitor metrics
- 50% → New version
- Monitor again
- 100% → New version

## Troubleshooting

### Get Pod Status

```bash
kubectl get pods
kubectl describe pod proglog-0
kubectl logs proglog-0
kubectl logs proglog-0 --previous  # Previous crash
```

### Exec into Pod

```bash
kubectl exec -it proglog-0 -- /bin/sh
```

### Port Forward

```bash
kubectl port-forward proglog-0 8080:8080
```

### Debug Container

```bash
kubectl debug proglog-0 -it --image=busybox
```

## Key Takeaways

✅ StatefulSets provide stable identities and storage  
✅ Helm simplifies Kubernetes deployment  
✅ Kubernetes handles service discovery automatically  
✅ Production requires proper resource management  
✅ Monitoring and observability are critical  

## Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [StatefulSet Best Practices](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [Production Best Practices](https://learnk8s.io/production-best-practices)

---

**Congratulations! 🎉**

You've built a complete production-ready distributed log service from scratch. You now understand:
- Distributed systems fundamentals
- Consensus algorithms (Raft)
- Service discovery patterns
- Security (TLS, ACLs)
- Observability (logging, metrics, tracing)
- Cloud deployment (Kubernetes)

Continue exploring distributed systems by:
- Building your own projects
- Contributing to open source
- Reading academic papers
- Experimenting with other patterns

**Keep building!** 🚀