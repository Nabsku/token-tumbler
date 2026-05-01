# Contributing

## Prerequisites

- Go 1.26 or newer
- Docker for E2E tests
- Optional: `golangci-lint`, `govulncheck`, and `git-cliff`

## Setup

```sh
git clone https://github.com/nabsku/token-tumbler.git
cd token-tumbler
go mod download
```

## Validation

Run the fast checks before committing:

```sh
make check
```

This runs formatting, unit tests, vet, build, and whitespace checks.

Optional checks:

```sh
make lint   # requires golangci-lint
make vuln   # requires govulncheck
make e2e    # requires Docker; starts GitLab CE and Vault
```

## Development Guidelines

- Keep changes small and commit atomically.
- Prefer conventional commit prefixes (`feat:`, `fix:`, `docs:`, `refactor:`, `test:`, `chore:`) so `git-cliff` can generate useful changelog entries.
- Return errors with context instead of panicking or logging and returning.
- Add or update tests for behavior changes.
- Keep unit tests independent of external services; use the `e2e` build tag for container-backed tests.
- Treat `config.example.yaml` as the tracked example only. Never commit real `config.yaml` files, tokens, Vault credentials, secret-store paths, or production target names.

## Changelog

`CHANGELOG.md` is generated from Git history with [`git-cliff`](https://git-cliff.org/):

```sh
make changelog
```

Before cutting a release, regenerate the changelog, review the output, commit it, then create and push the version tag.

## Pull Request Checklist

- [ ] `make check` passes
- [ ] Relevant tests were added or updated
- [ ] E2E tests were run or explicitly skipped with a reason
- [ ] Documentation was updated for user-facing behavior changes
- [ ] `CHANGELOG.md` was regenerated with `make changelog` for release PRs
