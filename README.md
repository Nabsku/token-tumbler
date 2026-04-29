# Token Chaser

Token Chaser is a Go daemon that rotates GitLab project and group access tokens and writes newly-created token values to a configured secret store. It currently supports GitLab project/group access tokens and Vault KVv2 persistence.

## Getting Started

Requirements:

- Go 1.25 or newer
- A GitLab token with permission to list, create, and revoke project/group access tokens
- Vault AppRole credentials when any repository entry uses `secretStore: vault`
- Docker for the optional Testcontainers E2E suite

Run locally:

```sh
export GITLAB_URL="https://gitlab.example.com"
export GITLAB_TOKEN="glpat-..."

# Only needed when at least one config entry uses secretStore: vault.
export APPROLE_ID="..."
export APPROLE_SECRET="..."

go run .
```

Token Chaser reads `config.yaml` from the current working directory.

## Configuration

Example:

```yaml
prefix: tc-
repositories:
  - repoName: example-project
    name: deploy
    permissions:
      - api
    rotationThreshold: 3d
    lifetime: 5d
    gracePeriod: 2d
    secretStore: vault
    vaultPath: teams/example/project
    vaultKey: gitlab_token
    vaultMount: kv
```

Each repository entry must define exactly one of `repoName` or `groupName`.

Required fields per entry:

- `name`: logical token name used in generated GitLab token names
- `permissions`: GitLab scopes for the token
- `rotationThreshold`: how soon before expiry a token should be renewed
- `lifetime`: maximum lifetime for newly-created tokens
- `gracePeriod`: how long to keep older tokens after a newer token exists
- `secretStore`: `vault` or `none`; use `none` explicitly when you do not want persistence

Duration suffixes: `s`, `m`, `h`, `d`, `w`, `M`.

Vault entries also require:

- `vaultPath`
- `vaultKey`
- `vaultMount`

## Development

Fast validation:

```sh
go test ./...
go vet ./...
go build ./...
```

Or use the Makefile:

```sh
make check
```

The slow E2E suite starts GitLab CE and Vault with Testcontainers:

```sh
go test -tags=e2e -v ./e2e -timeout 30m
```

Optional E2E image overrides:

- `TOKEN_CHASER_E2E_GITLAB_IMAGE`
- `TOKEN_CHASER_E2E_VAULT_IMAGE`

## Security Notes

- Generated GitLab token values are only available at creation time, so secret writes must succeed before old tokens are deleted.
- Unsupported or missing secret stores fail closed; use `secretStore: none` only for intentional no-persistence runs.
- Do not commit real GitLab tokens, Vault AppRole credentials, or production config secrets.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).
