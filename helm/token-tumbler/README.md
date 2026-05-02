# token-tumbler Helm chart

This chart runs Token Tumbler as a Kubernetes worker that rotates GitLab project and group access tokens.

## Install

From GHCR OCI:

```sh
helm install token-tumbler oci://ghcr.io/nabsku/charts/token-tumbler \
  --version <version> \
  -f values.yaml
```

From a local checkout:

```sh
helm install token-tumbler ./helm/token-tumbler \
  -f values.yaml
```

For production, avoid passing secrets with `--set` or committing them to values files. Create a Kubernetes Secret, or use an external secrets operator, and point the chart at it with `existingSecret`. When `existingSecret` is set, keep `env.gitlabToken` out of `values.yaml`; use the values file for non-secret `config` only.

```sh
kubectl create secret generic token-tumbler-env \
  --from-literal=GITLAB_URL="https://gitlab.example.com" \
  --from-literal=GITLAB_TOKEN="glpat-..."

helm install token-tumbler ./helm/token-tumbler \
  -f values.yaml \
  --set existingSecret=token-tumbler-env
```

## Required values

At minimum, set GitLab credentials and one managed target under `config.repositories`.

```yaml
env:
  gitlabUrl: https://gitlab.example.com
  gitlabToken: glpat-...

config:
  prefix: tt-
  repositories:
    - repoName: group/example-project
      name: deploy
      permissions:
        - read_repository
      accessLevel: 20
      rotationThreshold: 3d
      lifetime: 5d
      gracePeriod: 2d
      secretStore: vault
      vaultMount: kv
      vaultPath: teams/example/project
      vaultKey: gitlab_token
```

## Existing Secret keys

When `existingSecret` is set, the chart reads environment variables from that Secret. These keys are supported:

| Key | When needed |
| --- | --- |
| `GITLAB_URL` | Always |
| `GITLAB_TOKEN` | Always |
| `TOKEN_TUMBLER_INTERVAL` | Optional |
| `APPROLE_ID` | Vault AppRole auth |
| `APPROLE_SECRET` | Vault AppRole auth |
| `VAULT_TOKEN` | Vault token auth |
| `VAULT_K8S_TOKEN_PATH` | Optional Vault Kubernetes auth override |

## Metrics and probes

Metrics are enabled by default on port `9090` and expose:

- `GET /metrics`
- `GET /healthz`

If you use the Prometheus Operator, enable `metrics.serviceMonitor.enabled`.

`metrics.enabled: false` removes the chart's metrics port, Service, ServiceMonitor, and `TOKEN_TUMBLER_METRICS_ADDR` override. The app still starts its internal HTTP server on the default `:9090`; do not expose that port if you want metrics unavailable in Kubernetes.

```yaml
metrics:
  enabled: true
  service:
    enabled: true
  serviceMonitor:
    enabled: true
```

If `networkPolicy.enabled` is true, metrics ingress stays blocked unless `networkPolicy.metricsFrom` allows your monitoring namespace or pods.

Startup and liveness probes use the HTTP `/healthz` endpoint. Keep `metrics.enabled: true` when probes are enabled. Exec probes are not supported by this chart.

## Kubernetes Secret backend RBAC

The `k8s` secret store writes rotated token values into Kubernetes Secrets. The chart does not create that Secret read/write RBAC for you, because target namespaces vary by installation.

If you use `secretStore: k8s`, make sure the service account can read, create, and update the target Secret, and mount the service account token:

```yaml
serviceAccount:
  automount: true
config:
  repositories:
    - repoName: group/example-project
      name: deploy
      permissions:
        - read_repository
      accessLevel: 20
      rotationThreshold: 3d
      lifetime: 5d
      gracePeriod: 2d
      secretStore: k8s
      k8sNamespace: default
      k8sSecretName: gitlab-token
      k8sSecretKey: token
```

For each target namespace, add a Role/RoleBinding that grants `get`, `create`, and `update` on `secrets` to the Token Tumbler service account.

## Leader election and replica safety

Keep `replicaCount: 1` unless `leaderElection.enabled` is true. With leader election enabled, Token Tumbler uses a Kubernetes `Lease`; only the elected pod rotates tokens.

The chart fails to render if `replicaCount > 1` or `autoscaling.maxReplicas > 1` while leader election is disabled.

```yaml
replicaCount: 2
leaderElection:
  enabled: true
autoscaling:
  enabled: true
```

Leader election needs in-cluster Kubernetes credentials. When `leaderElection.enabled` and `leaderElection.rbac.create` are true, the chart mounts the service account token and creates namespace-scoped Lease RBAC.

## NetworkPolicy

`networkPolicy.enabled` defaults to false. When you enable it, the default egress allows DNS, HTTP, and HTTPS so the worker can reach GitLab, Vault, AWS, or Kubernetes APIs. Tighten those rules once you know the exact destinations in your cluster.

## Values reference

See [`values.yaml`](values.yaml) for all supported values and inline comments.
