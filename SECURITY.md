# Security Policy

## Reporting a Vulnerability

Please report suspected vulnerabilities privately. Do not open a public issue with exploit details, real tokens, Vault paths, GitLab project names, Kubernetes Secret names, AWS account details, or production configuration.

If GitHub private vulnerability reporting is enabled for this repository, use that channel. Otherwise, contact the maintainers directly and include:

- A short description of the issue and affected versions or commits.
- Steps to reproduce with sanitized examples only.
- The expected impact and any known mitigations.

We will acknowledge valid reports as soon as practical, investigate, and coordinate a fix before public disclosure.

## Sensitive Data Guidance

Token Tumbler manages GitLab access tokens and writes them to secret stores. When sharing logs, configs, screenshots, or bug reports, redact:

- GitLab personal, project, and group access tokens.
- Vault tokens, AppRole IDs/secrets, mount names, and secret paths.
- AWS account IDs, regions tied to private infrastructure, IAM role names, and Secrets Manager names.
- Kubernetes namespaces, Secret names, and service account tokens.
- Private GitLab group/project paths.

## Supported Versions

Security fixes are published through GitHub Releases. Users should run the latest released version unless a maintainer explicitly recommends otherwise.
