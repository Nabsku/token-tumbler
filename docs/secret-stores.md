# Secret Stores

Token Tumbler supports three secret store backends for persisting generated GitLab token values.

## Overview

| Store   | Description                                                                                                                                                                   |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `vault` | Writes the token value to Vault KVv2. Supports AppRole (default), direct token, Kubernetes, and AWS IAM auth. Existing secret data is merged so unrelated keys are preserved. |
| `file`  | Writes the token value to a local file using an atomic same-directory rename and `0600` permissions. The parent directory must already exist.                                 |
| `none`  | Does not persist the generated token. Use only when external persistence is intentionally handled elsewhere.                                                                  |

## Vault

The Vault secret store writes token values to HashiCorp Vault KVv2. It supports multiple authentication methods.

### AppRole (default)

The default auth method. Requires AppRole credentials:

```yaml
secretStore: vault
vaultAuthMethod: approle # optional, this is the default
vaultMount: kv
vaultPath: teams/example/project
vaultKey: gitlab_token
```

Environment variables:

- `APPROLE_ID` - Vault AppRole role ID
- `APPROLE_SECRET` - Vault AppRole secret ID

### Token auth

Use a direct Vault token:

```yaml
secretStore: vault
vaultAuthMethod: token
vaultMount: kv
vaultPath: teams/example/project
vaultKey: gitlab_token
```

Environment variables:

- `VAULT_TOKEN` - Direct Vault token

### Kubernetes auth

Authenticate using a Kubernetes service account token:

```yaml
secretStore: vault
vaultAuthMethod: kubernetes
vaultAuthRole: my-k8s-role
vaultMount: kv
vaultPath: teams/example/project
vaultKey: gitlab_token
```

Environment variables:

- `VAULT_K8S_TOKEN_PATH` - Optional path to Kubernetes service account token. Defaults to in-cluster path.

The Kubernetes auth method reads the service account token from the standard Kubernetes location (`/var/run/secrets/kubernetes.io/serviceaccount/token`) unless overridden.

### AWS IAM auth

Authenticate using AWS IAM credentials:

```yaml
secretStore: vault
vaultAuthMethod: aws
vaultAuthRole: my-aws-role
vaultMount: kv
vaultPath: teams/example/project
vaultKey: gitlab_token
```

No additional environment variables are required. The AWS auth method uses the standard AWS credential chain (environment variables, IAM instance profile, etc.).

### Merge behavior

All Vault writes use KVv2 and merge the new token value into existing secret data. Unrelated keys in the secret are preserved. Only the configured `vaultKey` is overwritten.

## File

The file secret store writes token values to a local file with atomic operations:

```yaml
secretStore: file
filePath: /run/secrets/gitlab-token
```

### Security considerations

- Files are created with `0600` permissions (owner read/write only)
- Writes are atomic: a temporary file is created in the same directory, then renamed over the target
- The parent directory must already exist
- File storage is only as safe as the host filesystem
- Prefer tmpfs or encrypted disks where appropriate
- Protect parent directory permissions
- Never commit generated token files to version control

## None

Use `none` when Token Tumbler should create/renew tokens but not persist the values:

```yaml
secretStore: none
```

This is useful when:

- External tooling handles secret persistence
- You want to test token creation without writing secrets
- Tokens are consumed immediately by another process

**Warning**: With `secretStore: none`, the daemon cannot recover token values after creation. Ensure you have another mechanism to capture the token if needed.
