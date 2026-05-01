# Changelog

All notable user-facing changes are documented here and in [GitHub Releases](https://github.com/Nabsku/token-tumbler/releases).

This project follows semantic versioning for tagged releases where practical.

## [Unreleased]

- Documentation improvements for public installation, Helm usage, monitoring, and support guidance.

## [1.0.1] - 2026-05-01

- Refactored the application entrypoint into focused internal packages.
- Kept `main.go` as a thin wiring layer for configuration, GitLab setup, metrics, and the runner loop.
- Moved tests alongside the packages they validate.

## [1.0.0] - 2026-05-01

- Prepared the repository for public release.
- Added sanitized example configuration and ignored local runtime config.
- Added security reporting guidance and release-history scaffolding.
- Made vulnerability scanning blocking in CI.

[Unreleased]: https://github.com/Nabsku/token-tumbler/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/Nabsku/token-tumbler/releases/tag/v1.0.1
[1.0.0]: https://github.com/Nabsku/token-tumbler/releases/tag/v1.0.0
