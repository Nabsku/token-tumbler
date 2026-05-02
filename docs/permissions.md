# Permissions and token keys

Token Tumbler uses one token to do the rotating, then creates other tokens for your apps or jobs to use. Those are easy to mix up.

| Setting | Meaning | Use |
| --- | --- | --- |
| `GITLAB_TOKEN` | The token Token Tumbler runs with. It lists existing project or group access tokens, creates new ones, and deletes old ones. | Use a bot or service account token with `api`. Give that account access only to the projects or groups in your config. |
| `config.repositories[].permissions` | The scopes on the tokens Token Tumbler creates. | Pick the scopes the consuming app actually needs. Avoid `api` unless that app needs API access. |
| `config.repositories[].accessLevel` | The GitLab role on the tokens Token Tumbler creates. | Set the lowest role that works. `30` means Developer. Omit it to use GitLab's default. |
| `vaultKey`, `k8sSecretKey`, or `filePath` | Where Token Tumbler writes the new token value. | Use the key or path your app already reads, for example `gitlab_token` or `token`. |

## `GITLAB_TOKEN`

`GITLAB_TOKEN` needs enough access to manage tokens for every target in `config.yaml`:

- `repoName` targets: the token owner needs Maintainer or Owner on the project.
- `groupName` targets: the token owner needs Owner on the group.
- The token needs the `api` scope. GitLab's token management API requires it.

Use a dedicated bot or service account if you can. A human PAT works, but it is harder to audit and easier to over-permission by accident.

## Scopes for generated tokens

The `permissions` field is for the tokens Token Tumbler creates, not for `GITLAB_TOKEN` itself:

```yaml
repositories:
  - repoName: group/example-project
    name: deploy
    permissions:
      - read_repository
    accessLevel: 20
    rotationThreshold: 3d
    lifetime: 5d
    gracePeriod: 2d
    secretStore: vault
    vaultMount: kv
    vaultPath: teams/example/project
    vaultKey: gitlab_token
```

Some common choices:

| App or job needs to | Scope |
| --- | --- |
| Clone or fetch source | `read_repository` |
| Pull from a registry | `read_registry` |
| Push to a repo or registry | `write_repository`, `write_registry` |
| Call GitLab APIs | `api` |

Start narrow. `api` is convenient, but it is usually more than a deploy key, CI job, or sync process needs.

## Access level for generated tokens

`accessLevel` maps to GitLab's role numbers:

| Value | Role |
| --- | --- |
| `10` | Guest |
| `20` | Reporter |
| `30` | Developer |
| `40` | Maintainer |
| `50` | Owner |

Use the lowest role that still lets the consuming app work. For many read-only jobs, `20` with `read_repository` is enough. GitLab may reject roles that are not allowed for a project or group token on your plan or target type.

## Destination keys

For Vault, `vaultKey` is the field Token Tumbler updates inside the KVv2 secret:

```yaml
secretStore: vault
vaultMount: kv
vaultPath: teams/example/project
vaultKey: gitlab_token
```

That writes `gitlab_token` under `kv/teams/example/project`. Other fields in the same secret stay untouched.

For Kubernetes Secrets, the equivalent field is `k8sSecretKey`:

```yaml
secretStore: k8s
k8sNamespace: default
k8sSecretName: gitlab-token
k8sSecretKey: token
```

## `glab` checks

Log in with the same token you plan to pass as `GITLAB_TOKEN`:

```sh
glab auth login --hostname gitlab.example.com --token "$GITLAB_TOKEN"
```

Check which user the token belongs to:

```sh
glab api user --hostname gitlab.example.com
```

List project access tokens:

```sh
glab api "projects/group%2Fexample-project/access_tokens" --hostname gitlab.example.com
```

Create a short lived project token as a smoke test:

```sh
glab api "projects/group%2Fexample-project/access_tokens" \
  --hostname gitlab.example.com \
  --method POST \
  --field name=tt-smoke-test \
  --field access_level=30 \
  --field expires_at=2026-12-31 \
  --field 'scopes[]=read_repository'
```

For a group target, use the group endpoint:

```sh
glab api "groups/group%2Fexample-group/access_tokens" \
  --hostname gitlab.example.com \
  --method POST \
  --field name=tt-smoke-test \
  --field access_level=30 \
  --field expires_at=2026-12-31 \
  --field 'scopes[]=read_repository'
```

Delete the smoke-test token afterwards. Replace `<token-id>` with the ID returned by the create or list command:

```sh
glab api "projects/group%2Fexample-project/access_tokens/<token-id>" \
  --hostname gitlab.example.com \
  --method DELETE
```

GitLab paths in API URLs need URL encoding. `group/example-project` becomes `group%2Fexample-project`.
