# Token Tumbler

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

Create `config.yaml`:

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

## Documentation

- **[Configuration](docs/configuration.md)** - Full configuration reference with all fields, validation rules, and examples
- **[Secret Stores](docs/secret-stores.md)** - Detailed docs for Vault, AWS, Kubernetes, file store, and `none`
- **[Monitoring](docs/monitoring.md)** - Prometheus metrics, PromQL queries, and alerting examples
- **[Development](docs/development.md)** - Running tests, E2E suite, Makefile targets, and contributing guidelines

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
