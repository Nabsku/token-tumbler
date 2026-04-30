# Secret Stores

Token Tumbler supports five secret store backends for persisting generated GitLab token values.

## Overview

| Store   | Description                                                                                                                                                                   |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `vault` | Writes the token value to Vault KVv2. Supports AppRole (default), direct token, Kubernetes, and AWS IAM auth. Existing secret data is merged so unrelated keys are preserved. |
| `file`  | Writes the token value to a local file using an atomic same-directory rename and `0600` permissions. The parent directory must already exist.                                 |
| `aws`   | Writes the token value to AWS Secrets Manager. Uses the standard AWS credential chain.                                                                                        |
| `k8s`   | Writes the token value to a Kubernetes Secret. Uses in-cluster config or kubeconfig. Other keys in the secret are preserved.                                                  |
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

## AWS Secrets Manager

The AWS secret store writes token values to AWS Secrets Manager:

```yaml
secretStore: aws
awsSecretName: my-gitlab-token
awsRegion: us-east-1
```

### Authentication

The AWS secret store uses the standard AWS credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (when running on EC2, ECS, or Lambda)

No additional configuration is required beyond ensuring AWS credentials are available.

### Behavior

- Creates a new secret version on each write
- The secret must already exist in AWS Secrets Manager
- Uses the AWS SDK for Go v2

## Kubernetes Secrets

The Kubernetes secret store writes token values to a Kubernetes Secret in a specified namespace:

```yaml
secretStore: k8s
k8sNamespace: default
k8sSecretName: gitlab-token
k8sSecretKey: token
```

### Authentication

The Kubernetes secret store automatically detects the execution environment:

1. **In-cluster**: Uses the service account token mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token`
2. **Outside cluster**: Loads kubeconfig from the default location (`~/.kube/config`) or via the `KUBECONFIG` environment variable

No additional configuration is required when running inside a Kubernetes pod with appropriate RBAC.

### RBAC

The service account used by Token Tumbler needs permission to read and write secrets in the target namespace:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: default
  name: token-tumbler-secret-manager
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: default
  name: token-tumbler-secret-manager
subjects:
  - kind: ServiceAccount
    name: token-tumbler
    namespace: default
roleRef:
  kind: Role
  name: token-tumbler-secret-manager
  apiGroup: rbac.authorization.k8s.io
```

### Behavior

- Creates the secret if it does not exist
- Merges the token value into existing secret data; other keys are preserved
- Uses the `Opaque` secret type
- Requires `k8sNamespace`, `k8sSecretName`, and `k8sSecretKey`

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
