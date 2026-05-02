# Secret stores

Token Tumbler can store generated GitLab token values in five places.

## Overview

| Store   | Description                                                                                                                                                                   |
| ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `vault` | Writes the token value to Vault KVv2. Supports AppRole (default), direct token, Kubernetes, and AWS IAM auth. Existing secret data is merged, so unrelated keys stay in place. |
| `file`  | Writes the token value to a local file using an atomic same-directory rename and `0600` permissions. The parent directory must already exist.                                 |
| `awsSecretsManager` | Writes the token value to AWS Secrets Manager. Uses the standard AWS credential chain.                                                                            |
| `kubernetesSecret`  | Writes the token value to a Kubernetes Secret. Uses in-cluster config or kubeconfig. Other keys in the secret are preserved.                                      |
| `none`  | Does not persist the generated token. Use it only when something else captures the value.                                                                                    |

## Vault

The Vault secret store writes token values to HashiCorp Vault KVv2. It supports several auth methods.

### AppRole (default)

This is the default auth method. It needs AppRole credentials:

```yaml
destination:
  type: vault
  vault:
    mount: kv
    path: teams/example/project
    key: gitlab_token
    auth:
      method: approle # optional, this is the default
```

Environment variables:

- `VAULT_ADDR` - Vault server URL, for example `https://vault.example.com`
- `APPROLE_ID` - Vault AppRole role ID
- `APPROLE_SECRET` - Vault AppRole secret ID

### Token auth

Use a direct Vault token:

```yaml
destination:
  type: vault
  vault:
    mount: kv
    path: teams/example/project
    key: gitlab_token
    auth:
      method: token
```

Environment variables:

- `VAULT_ADDR` - Vault server URL, for example `https://vault.example.com`
- `VAULT_TOKEN` - Direct Vault token

### Kubernetes auth

Authenticate using a Kubernetes service account token:

```yaml
destination:
  type: vault
  vault:
    mount: kv
    path: teams/example/project
    key: gitlab_token
    auth:
      method: kubernetes
      role: my-k8s-role
```

Environment variables:

- `VAULT_ADDR` - Vault server URL, for example `https://vault.example.com`
- `VAULT_K8S_TOKEN_PATH` - Optional path to Kubernetes service account token. Defaults to in-cluster path.

The Kubernetes auth method reads the service account token from the standard Kubernetes location (`/var/run/secrets/kubernetes.io/serviceaccount/token`) unless overridden.

### AWS IAM auth

Authenticate using AWS IAM credentials:

```yaml
destination:
  type: vault
  vault:
    mount: kv
    path: teams/example/project
    key: gitlab_token
    auth:
      method: aws
      role: my-aws-role
```

Set `VAULT_ADDR` to the Vault server URL. The AWS auth method uses the standard AWS credential chain, including environment variables and IAM instance profiles.

### Merge behavior

All Vault writes use KVv2 and merge the new token value into the existing secret data. Unrelated keys stay in place. Only the configured `destination.vault.key` is overwritten.

## File

The file secret store writes token values to a local file atomically:

```yaml
destination:
  type: file
  file:
    path: /run/secrets/gitlab-token
```

### Security considerations

- Files are created with `0600` permissions (owner read/write only)
- Writes are atomic: Token Tumbler creates a temporary file in the same directory, then renames it over the target
- The parent directory must already exist
- File storage is only as safe as the host filesystem
- Prefer tmpfs or encrypted disks when they fit your setup
- Protect parent directory permissions
- Never commit generated token files to version control

## AWS Secrets Manager

The AWS secret store writes token values to AWS Secrets Manager:

```yaml
destination:
  type: awsSecretsManager
  awsSecretsManager:
    secretName: my-gitlab-token
    region: us-east-1
```

### Authentication

The AWS secret store uses the standard AWS credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (when running on EC2, ECS, or Lambda)

No extra config is required beyond working AWS credentials.

### Behavior

- Creates a new secret version on each write
- Creates the main secret if it does not exist
- Stores token metadata in a second secret named `<destination.awsSecretsManager.secretName>-meta`, and creates it if it does not exist
- Needs IAM permissions for both names: `secretsmanager:GetSecretValue`, `secretsmanager:PutSecretValue`, and `secretsmanager:CreateSecret`
- Uses the AWS SDK for Go v2

## Kubernetes Secrets

The Kubernetes secret store writes token values to a Kubernetes Secret in a namespace:

```yaml
destination:
  type: kubernetesSecret
  kubernetesSecret:
    namespace: default
    name: gitlab-token
    key: token
```

### Authentication

The Kubernetes secret store detects where it is running:

1. In-cluster: uses the service account token mounted at `/var/run/secrets/kubernetes.io/serviceaccount/token`
2. Outside the cluster: loads kubeconfig from `~/.kube/config` or the `KUBECONFIG` environment variable

No extra config is required inside a Kubernetes pod with the right RBAC.

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
- Merges the token value into existing secret data; other keys stay in place
- Uses the `Opaque` secret type
- Requires `destination.kubernetesSecret.namespace`, `name`, and `key`

## None

Use `none` when Token Tumbler should create or renew tokens but not persist the values:

```yaml
destination:
  type: none
```

This can make sense when:

- External tooling handles secret persistence
- You want to test token creation without writing secrets
- Tokens are consumed immediately by another process

Warning: with `destination.type: none`, the daemon cannot recover token values after creation. Make sure another process captures the token if you need it later.
