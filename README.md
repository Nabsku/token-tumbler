# Token Tumbler

[![CI](https://github.com/Nabsku/token-tumbler/actions/workflows/ci.yml/badge.svg)](https://github.com/Nabsku/token-tumbler/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Nabsku/token-tumbler)](https://github.com/Nabsku/token-tumbler/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)
[![Go Reference](https://img.shields.io/badge/go-%3E%3D1.24-blue)](./go.mod)

Token Tumbler is a small Go daemon for safely rotating GitLab project and group access tokens. It creates replacement tokens before expiry, writes newly-created token values to a configured secret store, and only revokes older tokens after persistence succeeds.

It supports GitLab project/group access tokens and multiple secret stores: Vault KVv2 (with several auth methods), AWS Secrets Manager, Kubernetes Secrets, local file, or none.

## Why Token Tumbler?

- **Automated rotation** for GitLab project and group access tokens
- **Fail-closed secret handling** so token cleanup does not happen unless the new secret is safely stored
- **Multiple secret stores** — Vault KVv2 (AppRole, token, Kubernetes, AWS IAM), AWS Secrets Manager, Kubernetes Secrets, local file, or none
- **Merge-friendly writes** — existing secret data is preserved when updating Vault or Kubernetes secrets
- **Project and group targets** from one declarative YAML config
- **Grace-period cleanup** to keep the newest token alive while retiring stale tokens
- **Prometheus metrics** with token rotation counters, duration histograms, and a /healthz endpoint
- **Daemon mode** with a configurable polling interval and graceful shutdown
- **E2E coverage** with Testcontainers-backed GitLab and Vault

## Architecture

```mermaid
flowchart LR
    Config[config.yaml] --> Daemon[Token Tumbler daemon]
    Env[Environment variables] --> Daemon

    Daemon --> Validator[Config validation]
    Validator --> GitLab[GitLab API]

    GitLab --> Projects[Project access tokens]
    GitLab --> Groups[Group access tokens]

    Daemon --> Secrets{Secret store}
    Secrets --> Vault[Vault KVv2]
    Secrets --> AWS[AWS Secrets Manager]
    Secrets --> K8s[Kubernetes Secrets]
    Secrets --> File[file]
    Secrets --> None[none]

    Daemon --> Metrics[Prometheus metrics]
    Metrics --> Health[/healthz]
    Metrics --> MetricsEndpoint[/metrics]

    Daemon --> Cleanup[Old-token cleanup]
    Cleanup --> GitLab
```

See [docs/architecture.md](docs/architecture.md) for the rotation flow and safety model details.

## Quick start

Create `config.yaml` from the example:

```sh
cp config.example.yaml config.yaml
```

Then edit the targets and secret-store paths for your environment:

```yaml
prefix: tt-
repositories:
  - repoName: group/example-project
    name: deploy
    permissions:
      - api
    rotationThreshold: 3d
    lifetime: 5d
    gracePeriod: 2d
    secretStore: vault
    vaultMount: kv
    vaultPath: teams/example/project
    vaultKey: gitlab_token
```

Run the daemon:

```sh
export GITLAB_URL="https://gitlab.example.com"
export GITLAB_TOKEN="glpat-..."

# Optional; defaults to 5m. Uses Go duration syntax, for example 30s, 5m, 1h.
export TOKEN_TUMBLER_INTERVAL="5m"

# Vault AppRole (default auth method)
export APPROLE_ID="..."
export APPROLE_SECRET="..."

go run .
```

Other Vault auth methods:

```sh
# Direct token auth
export VAULT_TOKEN="hvs.XXXX"

# Kubernetes auth (reads service account token automatically)
export VAULT_K8S_TOKEN_PATH="/var/run/secrets/..."  # optional

# AWS IAM auth (uses standard AWS credential chain)
# No extra env vars needed; ensure AWS credentials are available
```

## Install and deploy

Token Tumbler reads `config.yaml` from the current working directory or, for the published container image, from `/config.yaml`.

### Release binary

Download a platform archive from [GitHub Releases](https://github.com/Nabsku/token-tumbler/releases), then run the binary next to your `config.yaml`:

```sh
tar -xzf token-tumbler_<version>_<os>_<arch>.tar.gz
GITLAB_URL="https://gitlab.example.com" \
GITLAB_TOKEN="glpat-..." \
./token-tumbler
```

### Docker

```sh
docker run --rm \
  -v "$PWD/config.yaml:/config.yaml:ro" \
  -e GITLAB_URL="https://gitlab.example.com" \
  -e GITLAB_TOKEN="glpat-..." \
  -p 9090:9090 \
  ghcr.io/nabsku/token-tumbler:latest
```

### Docker Compose

```sh
cp config.example.yaml config.yaml
export GITLAB_URL="https://gitlab.example.com"
export GITLAB_TOKEN="glpat-..."
docker compose up
```

### Helm

The Kubernetes chart lives in [`helm/token-tumbler`](helm/token-tumbler). Keep `replicaCount: 1` unless you add leader election or another external lock.

```sh
helm install token-tumbler ./helm/token-tumbler \
  --set env.gitlabUrl="https://gitlab.example.com" \
  --set env.gitlabToken="glpat-..."
```

For production, prefer `existingSecret` or an external secrets operator instead of passing secrets with `--set`. See the [Helm chart README](helm/token-tumbler/README.md).

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `GITLAB_URL` | Yes | GitLab base URL, for example `https://gitlab.example.com`. |
| `GITLAB_TOKEN` | Yes | GitLab token used to list, create, and revoke project/group access tokens. Grant only the minimum permissions needed for configured targets. |
| `TOKEN_TUMBLER_INTERVAL` | No | Polling interval. Defaults to `5m`. Uses Go duration syntax only, such as `30s`, `5m`, or `1h`. |
| `TOKEN_TUMBLER_METRICS_ADDR` | No | Metrics and health server bind address. Defaults to `:9090`. |
| `APPROLE_ID` | Vault AppRole only | Vault AppRole role ID. |
| `APPROLE_SECRET` | Vault AppRole only | Vault AppRole secret ID. |
| `VAULT_TOKEN` | Vault token auth only | Vault token for `vaultAuthMethod: token`. |
| `VAULT_K8S_TOKEN_PATH` | No | Optional service-account token path override for Vault Kubernetes auth. |

Config durations like `rotationThreshold`, `lifetime`, and `gracePeriod` support `s`, `m`, `h`, `d`, `w`, and `M`. `TOKEN_TUMBLER_INTERVAL` is different: it uses Go's `time.ParseDuration`, so use `s`, `m`, or `h`.

## Documentation

- **[Configuration](docs/configuration.md)** - Full configuration reference with all fields, validation rules, and examples
- **[Secret Stores](docs/secret-stores.md)** - Detailed docs for Vault, AWS, Kubernetes, file store, and `none`
- **[Monitoring](docs/monitoring.md)** - Prometheus metrics, PromQL queries, and alerting examples
- **[Development](docs/development.md)** - Running tests, E2E suite, Makefile targets, and contributing guidelines
- **[Helm Chart](helm/token-tumbler/README.md)** - Kubernetes install, values, metrics, and production notes

## Token naming

Generated tokens use this shape:

```text
<prefix><name>-<RFC3339 timestamp>
```

For example:

```text
tt-deploy-2026-04-29T12:00:00Z
```

`prefix` is normalized, so `tt` and `tt-` behave as the same prefix family for matching and cleanup.

## License

Token Tumbler is released under the [MIT License](./LICENSE).

## Getting help

Use [GitHub Issues](https://github.com/Nabsku/token-tumbler/issues) for bug reports and feature requests. Report security issues privately; see [SECURITY.md](SECURITY.md).
