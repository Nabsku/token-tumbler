# Security policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately. Do not open a public issue with exploit details, real tokens, Vault paths, GitLab project names, Kubernetes Secret names, AWS account details, or production config.

Use GitHub private vulnerability reporting when available:

<https://github.com/Nabsku/token-tumbler/security/advisories/new>

If that channel is unavailable, open a public issue without sensitive details and ask for a private maintainer contact. Include this much, and no more:

- A short description of the issue and affected versions or commits.
- Reproduction steps with sanitized examples only.
- Expected impact and any known mitigations.

We try to acknowledge reports within 7 days, give an initial assessment within 14 days, and coordinate a fix before public disclosure when the report is valid.

## Sensitive data

Token Tumbler manages GitLab access tokens and writes them to secret stores. Before sharing logs, configs, screenshots, or bug reports, redact:

- GitLab personal, project, and group access tokens.
- Vault tokens, AppRole IDs/secrets, mount names, and secret paths.
- AWS account IDs, regions tied to private infrastructure, IAM role names, and Secrets Manager names.
- Kubernetes namespaces, Secret names, and service account tokens.
- Private GitLab group/project paths.

## Supported versions

Security fixes ship through GitHub Releases. Run the latest release unless a maintainer says otherwise.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | Best effort only |
