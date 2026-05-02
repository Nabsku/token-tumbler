# Permissions and token keys

Token Tumbler deals with two different GitLab token concepts. Keep them separate when you design access:

| Setting | What it is | Recommended access |
| --- | --- | --- |
| `GITLAB_TOKEN` | The operator token Token Tumbler runs with. It lists existing project/group access tokens, creates replacements, and revokes stale tokens. | Use a dedicated bot or service account token with the `api` scope. Grant that account only the GitLab role required on the configured targets. |
| `config.repositories[].permissions` | The scopes granted to each newly created project/group access token. | Grant only what the consumer of that generated token needs. Do not default to `api` unless the downstream workload needs API write access. |
| `vaultKey`, `k8sSecretKey`, or `filePath` | The destination key/path where Token Tumbler writes the generated token value. | Use a stable key name expected by the consuming workload, such as `gitlab_token` or `token`. |

## `GITLAB_TOKEN` permissions

`GITLAB_TOKEN` must be able to manage access tokens for every configured target:

- For `repoName` targets, the token owner should have Maintainer or Owner access to the project.
- For `groupName` targets, the token owner should have Owner access to the group.
- The token itself needs the `api` scope because GitLab token management APIs require API access.

Use a dedicated bot/service account instead of a human user's personal token when possible. Limit that account to only the projects or groups Token Tumbler manages.

## Generated token scopes

The `permissions` field controls the scopes on tokens Token Tumbler creates:

```yaml
repositories:
  - repoName: group/example-project
    name: deploy
    permissions:
      - read_repository
    rotationThreshold: 3d
    lifetime: 5d
    gracePeriod: 2d
    secretStore: vault
    vaultMount: kv
    vaultPath: teams/example/project
    vaultKey: gitlab_token
```

Common examples:

| Consumer needs | Example generated token scopes |
| --- | --- |
| Clone or fetch source only | `read_repository` |
| Read container/package registries | `read_registry` |
| Push to repository or registry | `write_repository`, `write_registry` |
| Full API access | `api` |

Prefer narrower scopes than `api` for generated tokens. Use `api` only when the consuming workload really needs GitLab API access beyond repository or registry reads/writes.

## Destination keys

For secret stores that support structured secrets, the key controls where the generated token value is written:

```yaml
secretStore: vault
vaultMount: kv
vaultPath: teams/example/project
vaultKey: gitlab_token
```

This writes only the `gitlab_token` field inside the Vault KVv2 secret at `kv/teams/example/project`. Other keys in the same secret are preserved.

For Kubernetes Secrets, use `k8sSecretKey`:

```yaml
secretStore: k8s
k8sNamespace: default
k8sSecretName: gitlab-token
k8sSecretKey: token
```

## `glab` examples

Authenticate `glab` with the operator token you plan to use as `GITLAB_TOKEN`:

```sh
glab auth login --hostname gitlab.example.com --token "$GITLAB_TOKEN"
```

Check the authenticated user:

```sh
glab api user --hostname gitlab.example.com
```

List project access tokens for a project target:

```sh
glab api "projects/group%2Fexample-project/access_tokens" --hostname gitlab.example.com
```

Create a short-lived project access token manually to verify the operator token can manage project tokens:

```sh
glab api "projects/group%2Fexample-project/access_tokens" \
  --hostname gitlab.example.com \
  --method POST \
  --field name=tt-smoke-test \
  --field access_level=30 \
  --field expires_at=2026-12-31 \
  --field 'scopes[]=read_repository'
```

For group targets, use the group access token endpoint:

```sh
glab api "groups/group%2Fexample-group/access_tokens" \
  --hostname gitlab.example.com \
  --method POST \
  --field name=tt-smoke-test \
  --field access_level=30 \
  --field expires_at=2026-12-31 \
  --field 'scopes[]=read_repository'
```

Delete the smoke-test token afterwards. Use the token ID returned by the create/list command:

```sh
glab api "projects/group%2Fexample-project/access_tokens/<token-id>" \
  --hostname gitlab.example.com \
  --method DELETE
```

GitLab project and group paths must be URL-encoded in API paths. For example, `group/example-project` becomes `group%2Fexample-project`.
