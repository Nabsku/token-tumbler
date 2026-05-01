# Architecture

## Rotation flow

```mermaid
sequenceDiagram
    autonumber
    participant T as Token Tumbler
    participant C as config.yaml
    participant G as GitLab API
    participant S as Secret Store

    T->>C: Load and validate targets
    loop Every TOKEN_TUMBLER_INTERVAL
        T->>G: Resolve project or group
        T->>G: List active matching tokens
        alt No matching token exists
            T->>G: Create replacement token
            G-->>T: Token value, id, expiry
            T->>S: Persist token value
            S-->>T: Write success
            T->>G: Revoke stale prefixed tokens after grace period
        else All matching tokens are near expiry
            T->>G: Create renewed token
            G-->>T: Token value, id, expiry
            T->>S: Persist token value
            S-->>T: Write success
            T->>G: Revoke stale prefixed tokens after grace period
        else A healthy token exists
            T-->>T: Leave tokens unchanged
        end
    end
```

## Safety model

```mermaid
flowchart TD
    Start[Need create or renewal?] -->|No| Stop[Do nothing]
    Start -->|Yes| Create[Create new GitLab token]
    Create --> Store[Write token value to secret store]
    Store -->|Fails| Keep[Keep existing tokens and fail closed]
    Store -->|Succeeds| Delete[Delete only old active prefixed tokens]
    Delete --> Preserve[Always preserve newest token]
```

Token Tumbler is intentionally conservative:

- GitLab only shows a generated token value once, so the secret write must succeed before old tokens are revoked.
- Unsupported or missing secret stores fail closed.
- Cleanup only revokes prefixed, active, non-revoked tokens older than the configured grace period.
- The newest matching token is never revoked.
- Tokens with missing creation timestamps are never selected as the newest cleanup candidate.
- Duplicate config entries for the same prefix, target type, target, and token name are rejected.

## Observability

Token Tumbler exposes Prometheus metrics on a configurable HTTP endpoint. The default is `:9090`.

- `/metrics` - Prometheus metrics, including rotation counters, duration histograms, and secret store operation counters
- `/healthz` - health check endpoint

See [monitoring.md](monitoring.md) for metric names, PromQL queries, and alert examples.
