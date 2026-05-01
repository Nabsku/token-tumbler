# token-tumbler Helm chart

This chart deploys Token Tumbler as a Kubernetes worker for rotating GitLab project and group access tokens.

## Install

From a local checkout:

```sh
helm install token-tumbler ./helm/token-tumbler \
  --set env.gitlabUrl="https://gitlab.example.com" \
  --set env.gitlabToken="glpat-..."
```

For production, do not pass secrets with `--set`. Create a Kubernetes Secret or use an external secrets operator, then set `existingSecret`.

```sh
kubectl create secret generic token-tumbler-env \
  --from-literal=GITLAB_URL="https://gitlab.example.com" \
  --from-literal=GITLAB_TOKEN="glpat-..." \
  --from-literal=TOKEN_TUMBLER_INTERVAL="5m"

helm install token-tumbler ./helm/token-tumbler \
  --set existingSecret=token-tumbler-env
```

## Required values

At minimum, set GitLab credentials and at least one managed target under `config.repositories`.

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
        - api
      rotationThreshold: 3d
      lifetime: 5d
      gracePeriod: 2d
      secretStore: k8s
      k8sNamespace: default
      k8sSecretName: gitlab-token
      k8sSecretKey: token
```

## Existing Secret keys

When `existingSecret` is set, the chart reads environment variables from that Secret. Supported keys are:

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

Enable `metrics.serviceMonitor.enabled` when using the Prometheus Operator.

```yaml
metrics:
  enabled: true
  service:
    enabled: true
  serviceMonitor:
    enabled: true
```

If `networkPolicy.enabled` is true, metrics ingress remains denied unless `networkPolicy.metricsFrom` allows your monitoring namespace or pods.

## Replica safety

Keep `replicaCount: 1`. Multiple replicas can rotate the same token concurrently because the application does not currently include leader election or a distributed lock.

```yaml
replicaCount: 1
autoscaling:
  enabled: false
```

## NetworkPolicy

`networkPolicy.enabled` defaults to false. When enabled, the default egress allows DNS, HTTP, and HTTPS so the worker can reach GitLab, Vault, AWS, or Kubernetes APIs. Tighten these rules for your cluster once you know the required destinations.

## Values reference

See [`values.yaml`](values.yaml) for the full set of supported values and inline comments.
