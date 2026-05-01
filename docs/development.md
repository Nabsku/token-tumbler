# Development

## Requirements

- Go 1.26 or newer
- Docker (for the optional Testcontainers E2E suite)

## Fast validation

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./...
```

Or use the Makefile:

```sh
make check
```

## Makefile targets

| Target       | Description                                         |
| ------------ | --------------------------------------------------- |
| `make fmt`   | Format Go code.                                     |
| `make test`  | Run unit tests.                                     |
| `make vet`   | Run `go vet`.                                       |
| `make build` | Build the project.                                  |
| `make check` | Run formatting, tests, vet, build, and diff checks. |
| `make e2e`   | Run the GitLab/Vault Testcontainers suite.          |
| `make lint`  | Run lint checks.                                    |
| `make vuln`  | Run vulnerability checks.                           |
| `make tidy`  | Tidy Go modules.                                    |
| `make changelog` | Regenerate `CHANGELOG.md` with `git-cliff`.     |
| `make changelog-check` | Verify `CHANGELOG.md` matches `git-cliff` output. |
| `make install-hooks` | Install the local prek hooks. |

## Changelog generation

`CHANGELOG.md` is generated from conventional commits with [`git-cliff`](https://git-cliff.org/). Install `git-cliff`, then run:

```sh
make changelog
```

Use conventional commit prefixes so the changelog groups entries sensibly:

- `feat:` for user-facing features
- `fix:` for bug fixes
- `docs:` for documentation
- `refactor:` for internal restructuring
- `test:` for tests
- `chore:` for maintenance

Before publishing a release, regenerate and commit `CHANGELOG.md`, then create and push the release tag.

The prek hook runs `make changelog-check`. If the hook fails, run `make changelog` and include the updated `CHANGELOG.md` in your commit.

## E2E tests

The slow E2E suite starts GitLab CE and Vault with Testcontainers:

```sh
go test -tags=e2e -v ./e2e -timeout 30m
```

Optional E2E image overrides:

- `TOKEN_TUMBLER_E2E_GITLAB_IMAGE`
- `TOKEN_TUMBLER_E2E_VAULT_IMAGE`

## Contributing

Contributions are welcome. Run `make check` before opening a pull request, and do not commit real GitLab tokens, Vault credentials, secret store paths, private target names, or production config files.

See [CONTRIBUTING.md](../CONTRIBUTING.md).
