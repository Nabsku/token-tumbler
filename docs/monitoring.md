# Monitoring

Token Tumbler exposes Prometheus metrics and a health endpoint.

## Metrics endpoint

By default, Token Tumbler starts an HTTP server on `:9090`. Set `TOKEN_TUMBLER_METRICS_ADDR` to use another address.

### Endpoints

| Path       | Description                                           |
| ---------- | ----------------------------------------------------- |
| `/metrics` | Prometheus metrics in exposition format               |
| `/healthz` | Health check; returns `200 OK` with body `ok`         |

### Environment variables

| Variable                    | Default | Description                          |
| --------------------------- | ------- | ------------------------------------ |
| `TOKEN_TUMBLER_METRICS_ADDR`| `:9090` | Listen address for the HTTP server   |

## Prometheus metrics

All metrics use the `token_tumbler_` prefix.

### `token_tumbler_token_rotations_total`

Counts token rotation attempts by target type, repository name, secret store, and outcome.

| Label         | Values                                      |
| ------------- | ------------------------------------------- |
| `target_type` | `project`, `group`                          |
| `repo_name`   | The repository `name` from config           |
| `secret_store`| `vault`, `file`, `aws`, `k8s`, `none`       |
| `outcome`     | `success`, `error`                          |

### `token_tumbler_token_rotation_duration_seconds`

Histogram for token rotation duration.

| Label         | Values                            |
| ------------- | --------------------------------- |
| `target_type` | `project`, `group`                |
| `repo_name`   | The repository `name` from config |

### `token_tumbler_secret_store_operations_total`

Counts secret store writes.

| Label        | Values                                |
| ------------ | ------------------------------------- |
| `secret_store`| `vault`, `file`, `aws`, `k8s`, `none` |
| `operation`  | `write`                               |
| `outcome`    | `success`, `error`                    |

### `token_tumbler_active_tokens`

Number of active tokens found for each target at the start of a poll cycle.

| Label         | Values                            |
| ------------- | --------------------------------- |
| `target_type` | `project`, `group`                |
| `repo_name`   | The repository `name` from config |

### `token_tumbler_token_rollback_attempts_total`

Counts rollback attempts after Token Tumbler created a replacement token but failed to store it.

| Label         | Values             |
| ------------- | ------------------ |
| `target_type` | `project`, `group` |
| `repo_name`   | The repository `name` from config |

### `token_tumbler_token_rollback_outcomes_total`

Counts whether those rollback attempts worked.

| Label         | Values             |
| ------------- | ------------------ |
| `target_type` | `project`, `group` |
| `repo_name`   | The repository `name` from config |
| `outcome`     | `success`, `failure` |

### `token_tumbler_orphan_tokens_detected_total`

Counts cases where GitLab has a newer token than the one currently stored in Vault.

| Label         | Values             |
| ------------- | ------------------ |
| `target_type` | `project`, `group` |
| `repo_name`   | The repository `name` from config |

### `token_tumbler_cleanup_skipped_total`

Counts cleanup passes skipped because Vault metadata could not be read safely.

| Label         | Values             |
| ------------- | ------------------ |
| `target_type` | `project`, `group` |
| `repo_name`   | The repository `name` from config |

## Prometheus query examples

Rotation success rate over the last hour:
```promql
sum(rate(token_tumbler_token_rotations_total{outcome="success"}[1h]))
/
sum(rate(token_tumbler_token_rotations_total[1h]))
```

Average rotation duration:
```promql
histogram_quantile(0.95,
  rate(token_tumbler_token_rotation_duration_seconds_bucket[5m])
)
```

Failed secret store writes:
```promql
rate(token_tumbler_secret_store_operations_total{outcome="error"}[5m])
```

Active tokens per target:
```promql
token_tumbler_active_tokens
```

Rollback errors:
```promql
rate(token_tumbler_token_rollback_outcomes_total{outcome="failure"}[5m])
```

Orphan token detections:
```promql
increase(token_tumbler_orphan_tokens_detected_total[1h])
```

Skipped cleanup:
```promql
increase(token_tumbler_cleanup_skipped_total[1h])
```

## Alert examples

High rotation failure rate:
```yaml
- alert: TokenTumblerRotationFailures
  expr: |
    sum(rate(token_tumbler_token_rotations_total{outcome="error"}[5m]))
    /
    sum(rate(token_tumbler_token_rotations_total[5m])) > 0.1
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Token Tumbler rotation failure rate is high"
```

Secret store write failures:
```yaml
- alert: TokenTumblerSecretStoreFailures
  expr: rate(token_tumbler_secret_store_operations_total{outcome="error"}[5m]) > 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Token Tumbler is failing to write secrets"
```

Rollback failures:
```yaml
- alert: TokenTumblerRollbackFailures
  expr: rate(token_tumbler_token_rollback_outcomes_total{outcome="failure"}[5m]) > 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Token Tumbler failed to roll back a token after secret persistence failed"
```

Cleanup skipped:
```yaml
- alert: TokenTumblerCleanupSkipped
  expr: increase(token_tumbler_cleanup_skipped_total[30m]) > 0
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Token Tumbler skipped old-token cleanup because secret metadata was unavailable"
```

## Kubernetes setup

In Kubernetes, the metrics server listens on the pod IP. Add a `ServiceMonitor` or `PodMonitor` to scrape it:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: token-tumbler
  labels:
    app: token-tumbler
spec:
  selector:
    matchLabels:
      app: token-tumbler
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
```

Make sure the service exposes the metrics port:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: token-tumbler
  labels:
    app: token-tumbler
spec:
  selector:
    app: token-tumbler
  ports:
    - name: metrics
      port: 9090
      targetPort: 9090
```
