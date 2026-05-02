# Configuration reference

Token Tumbler uses one YAML file, `config.yaml`. The schema is also available as [`config.schema.json`](../config.schema.json) for editor validation.

Add this line to the top of your config file if your editor supports the YAML language server:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/Nabsku/token-tumbler/main/config.schema.json
```

## Migration from the old flat config

The current config shape replaces the older flat `repositories` list. Use this mapping when updating a pre-1.2 config:

| Old field | New field |
| --- | --- |
| `prefix` | `token.prefix` |
| `repositories[]` | `targets[]` |
| `repoName` | `gitlab.type: project` and `gitlab.path` |
| `groupName` | `gitlab.type: group` and `gitlab.path` |
| `permissions` | `generatedToken.scopes` |
| `lifetime` | `generatedToken.lifetime` |
| `rotationThreshold` | `rotation.threshold` |
| `gracePeriod` | `rotation.gracePeriod` |
| `secretStore` and backend fields | `destination.type` and the matching `destination.<backend>` block |

For example, `secretStore: vault`, `vaultMount`, `vaultPath`, and `vaultKey` become:

```yaml
destination:
  type: vault
  vault:
    mount: kv
    path: teams/example/project
    key: gitlab_token
```

## Example

```yaml
token:
  prefix: tt-

gitlab:
  url: https://gitlab.example.com

targets:
  - name: deploy
    gitlab:
      type: project
      path: group/example-project
    generatedToken:
      scopes:
        - read_repository
      accessLevel: reporter
      lifetime: 5d
    rotation:
      threshold: 3d
      gracePeriod: 2d
    destination:
      type: vault
      vault:
        mount: kv
        path: teams/example/project
        key: gitlab_token
```

## Top-level fields

| Field | Required | Description |
| --- | --- | --- |
| `gitlab.url` | No | GitLab base URL. Use this for non-secret config, or set `GITLAB_URL` through the environment/Helm Secret. |
| `token.prefix` | Yes | Prefix for generated GitLab token names. Allowed characters: letters, numbers, `-`, `_`. |
| `targets` | Yes | One or more project or group token targets. |

## Target fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | Yes | Logical token name used in generated GitLab token names. |
| `gitlab.type` | Yes | `project` or `group`. |
| `gitlab.path` | One of `path` or `id` | GitLab project or group path. |
| `gitlab.id` | One of `path` or `id` | GitLab numeric project or group ID, encoded as a string. Use `id: "12345"`, not `id: 12345`. |
| `generatedToken.scopes` | Yes | Scopes for generated tokens, such as `read_repository` or `api`. |
| `generatedToken.accessLevel` | No | `guest`, `reporter`, `developer`, `maintainer`, or `owner`. Omit it to use GitLab's default. |
| `generatedToken.lifetime` | Yes | Maximum lifetime for new tokens. Must be greater than `rotation.threshold`. |
| `rotation.threshold` | Yes | How soon before expiry a token should be renewed. |
| `rotation.gracePeriod` | Yes | How long to keep older tokens after a newer token exists. May be `0`. |
| `destination.type` | Yes | `vault`, `awsSecretsManager`, `kubernetesSecret`, `file`, or `none`. |

## Destination fields

### Vault

```yaml
destination:
  type: vault
  vault:
    mount: kv
    path: teams/example/project
    key: gitlab_token
    auth:
      method: approle # approle, token, kubernetes, or aws
      role: my-role   # required for kubernetes and aws auth
```

### AWS Secrets Manager

```yaml
destination:
  type: awsSecretsManager
  awsSecretsManager:
    region: us-east-1
    secretName: my-gitlab-token
```

### Kubernetes Secret

```yaml
destination:
  type: kubernetesSecret
  kubernetesSecret:
    namespace: default
    name: gitlab-token
    key: token
```

### File

```yaml
destination:
  type: file
  file:
    path: /run/secrets/gitlab-token
```

### None

```yaml
destination:
  type: none
```

## Durations

Config duration suffixes: `s`, `m`, `h`, `d`, `w`, `M` (`M` is 30 days). `TOKEN_TUMBLER_INTERVAL` uses Go duration syntax instead, so use values like `30s`, `5m`, or `1h` there.

## Uniqueness rules

Token targets must be unique by `token.prefix`, `gitlab.type`, GitLab target path or ID, and `name`. That keeps two config entries from managing the same GitLab token family.

## Environment variables

| Variable | Required | Description |
| --- | --- | --- |
| `GITLAB_URL` | No | GitLab base URL. Required only when `gitlab.url` is not set in config. |
| `GITLAB_TOKEN` | Yes | GitLab token used to list, create, and revoke configured project/group access tokens. See [Permissions and token keys](permissions.md). |
| `TOKEN_TUMBLER_INTERVAL` | No | Polling interval. Defaults to `5m`. Uses Go duration syntax (`30s`, `5m`, `1h`). |
| `TOKEN_TUMBLER_METRICS_ADDR` | No | Metrics and health server bind address. Defaults to `:9090`. |
| `VAULT_ADDR` | Vault only | Vault server URL, for example `https://vault.example.com`. |
| `APPROLE_ID` | Vault AppRole only | Vault AppRole role ID. |
| `APPROLE_SECRET` | Vault AppRole only | Vault AppRole secret ID. |
| `VAULT_TOKEN` | Vault token auth only | Vault token for `auth.method: token`. |
| `VAULT_K8S_TOKEN_PATH` | No | Optional service-account token path override for Vault Kubernetes auth. |

Use leader election for Kubernetes deployments with more than one replica. It needs in-cluster Kubernetes credentials that can get, list, watch, create, update, and patch `coordination.k8s.io` Lease objects in the configured namespace.

## Validation

The daemon validates configuration on startup and exits if required fields are missing, durations are invalid, destination settings are incomplete, access level is unknown, or duplicate token targets exist.
