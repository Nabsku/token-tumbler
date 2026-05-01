# Security Policy

## Reporting a Vulnerability

Please report suspected vulnerabilities privately. Do not open a public issue with exploit details, real tokens, Vault paths, GitLab project names, Kubernetes Secret names, AWS account details, or production configuration.

Use GitHub private vulnerability reporting when available:

<https://github.com/Nabsku/token-tumbler/security/advisories/new>

If that channel is unavailable, open a public issue without sensitive details and ask for a private maintainer contact. Include:

- A short description of the issue and affected versions or commits.
- Steps to reproduce with sanitized examples only.
- The expected impact and any known mitigations.

We aim to acknowledge reports within 7 days, provide an initial assessment within 14 days, and coordinate a fix before public disclosure when the report is valid.

## Sensitive Data Guidance

Token Tumbler manages GitLab access tokens and writes them to secret stores. When sharing logs, configs, screenshots, or bug reports, redact:

- GitLab personal, project, and group access tokens.
- Vault tokens, AppRole IDs/secrets, mount names, and secret paths.
- AWS account IDs, regions tied to private infrastructure, IAM role names, and Secrets Manager names.
- Kubernetes namespaces, Secret names, and service account tokens.
- Private GitLab group/project paths.

## Supported Versions

Security fixes are published through GitHub Releases. Users should run the latest released version unless a maintainer explicitly recommends otherwise.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | Best effort only |
