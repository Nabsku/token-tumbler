# Configuration Reference

Token Tumbler uses a single declarative YAML file (`config.yaml`) to define all token rotation targets.

## Example

```yaml
prefix: tt-
repositories:
  - repoName: group/example-project
    name: deploy
    permissions:
      - api
    rotationThreshold: 3d
    lifetime: 5d
    gracePeriod: 2d
    secretStore: vault
    vaultMount: kv
    vaultPath: teams/example/project
    vaultKey: gitlab_token
```

## Top-level fields

| Field          | Required | Description                                                                              |
| -------------- | -------- | ---------------------------------------------------------------------------------------- |
| `prefix`       | Yes      | Prefix for generated GitLab token names. Allowed characters: letters, numbers, `-`, `_`. |
| `repositories` | Yes      | Non-empty list of project or group token targets.                                        |

## Repository fields

Each entry must define exactly one target (`repoName` or `groupName`):

| Field               | Required                         | Description                                                                          |
| ------------------- | -------------------------------- | ------------------------------------------------------------------------------------ |
| `repoName`          | One of `repoName` or `groupName` | GitLab project path or ID.                                                           |
| `groupName`         | One of `repoName` or `groupName` | GitLab group path or ID.                                                             |
| `name`              | Yes                              | Logical token name used in generated GitLab token names.                             |
| `permissions`       | Yes                              | GitLab token scopes, such as `api`.                                                  |
| `rotationThreshold` | Yes                              | How soon before expiry a token should be renewed.                                    |
| `lifetime`          | Yes                              | Maximum lifetime for newly-created tokens. Must be greater than `rotationThreshold`. |
| `gracePeriod`       | Yes                              | How long to keep older tokens after a newer token exists. May be `0`.                |
| `secretStore`       | Yes                              | `vault`, `file`, `aws`, `k8s`, or `none`. Use `none` only for intentional no-persistence runs. |
| `vaultMount`        | For Vault                        | Vault KVv2 mount name.                                                               |
| `vaultPath`         | For Vault                        | Vault KVv2 secret path.                                                              |
| `vaultKey`          | For Vault                        | Key inside the KVv2 secret data to write.                                            |
| `vaultAuthMethod`   | For Vault                        | Auth method: `approle` (default), `token`, `kubernetes`, or `aws`.                   |
| `vaultAuthRole`     | For k8s/AWS                      | Role name for Kubernetes or AWS auth.                                                |
| `filePath`          | For file                         | Destination path for the token file. Parent directory must already exist.            |
| `awsSecretName`     | For AWS                          | Name of the secret in AWS Secrets Manager.                                           |
| `awsRegion`         | For AWS                          | AWS region where the secret is stored.                                               |
| `k8sNamespace`      | For k8s                          | Kubernetes namespace where the secret lives.                                         |
| `k8sSecretName`     | For k8s                          | Name of the Kubernetes Secret resource.                                              |
| `k8sSecretKey`      | For k8s                          | Key inside the Kubernetes Secret data to write.                                      |

## Durations

Duration suffixes: `s`, `m`, `h`, `d`, `w`, `M` (`M` is 30 days).

Examples:

- `30s` - 30 seconds
- `5m` - 5 minutes
- `24h` - 1 day
- `7d` - 7 days
- `4w` - 4 weeks
- `6M` - 6 months (180 days)

## Uniqueness rules

Token targets must be unique by `prefix`, target type (`repoName` or `groupName`), target value, and `name`. This prevents two config entries from creating or cleaning up the same logical GitLab token.

## Validation

The daemon validates configuration on startup and fails fast if:

- The prefix contains invalid characters
- No repositories are defined
- Any repository is missing required fields
- Duration values are invalid or inconsistent (e.g., `rotationThreshold` >= `lifetime`)
- Secret store configuration is incomplete
- Duplicate token targets exist
