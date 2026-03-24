# Observability

Orchestrator provides Prometheus metrics and Kubernetes-ready health check endpoints for monitoring and integration with cloud-native infrastructure.

## Prometheus Metrics

When `PrometheusEnabled` is `true` (the default), orchestrator exposes a `/metrics` endpoint compatible with the Prometheus scraping format.

### Configuration

```json
{
  "PrometheusEnabled": true
}
```

Set `PrometheusEnabled` to `false` to disable the Prometheus metrics endpoint.

### Available Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `orchestrator_discoveries_total` | Counter | Total number of discovery attempts |
| `orchestrator_discovery_errors_total` | Counter | Total number of failed discoveries |
| `orchestrator_instances_total` | Gauge | Total number of known instances |
| `orchestrator_clusters_total` | Gauge | Total number of known clusters |
| `orchestrator_recoveries_total` | Counter | Recovery attempts (labels: `type`, `result`) |
| `orchestrator_recovery_duration_seconds` | Histogram | Duration of recovery operations |

### Example Prometheus scrape config

```yaml
scrape_configs:
  - job_name: orchestrator
    static_configs:
      - targets: ['orchestrator:3000']
    metrics_path: /metrics
    scrape_interval: 15s
```

## Health Check Endpoints

Three health check endpoints are provided for Kubernetes probes and load balancer health checks:

### `GET /health/live`

**Liveness probe.** Returns `200 OK` if the orchestrator process is running. This is a lightweight check that does not query any backend.

Response:
```json
{"status": "alive"}
```

### `GET /health/ready`

**Readiness probe.** Returns `200 OK` if the backend database is connected and health check registration is succeeding. Returns `503 Service Unavailable` otherwise.

Response (ready):
```json
{"status": "ready"}
```

Response (not ready):
```json
{"status": "not ready"}
```

### `GET /health/leader`

**Leader check.** Returns `200 OK` if this node is the raft leader (when raft is enabled) or the active/healthy node (when raft is disabled). Returns `503 Service Unavailable` otherwise.

This endpoint is useful for directing writes only to the leader node via a load balancer.

Response (leader):
```json
{"status": "leader"}
```

Response (not leader):
```json
{"status": "not leader"}
```

## Kubernetes Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orchestrator
spec:
  template:
    spec:
      containers:
        - name: orchestrator
          ports:
            - containerPort: 3000
          livenessProbe:
            httpGet:
              path: /health/live
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 3000
            initialDelaySeconds: 10
            periodSeconds: 5
```

For directing traffic only to the leader in a multi-node raft deployment, use the `/health/leader` endpoint with a Kubernetes `Service`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: orchestrator-leader
spec:
  selector:
    app: orchestrator
  ports:
    - port: 3000
---
# Use /health/leader as the readiness probe on the leader service
```
