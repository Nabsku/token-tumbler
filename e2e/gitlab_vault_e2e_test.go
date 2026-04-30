//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/nabsku/token-tumbler/internal/project"
	"github.com/nabsku/token-tumbler/internal/secrets"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcvault "github.com/testcontainers/testcontainers-go/modules/vault"
	"github.com/testcontainers/testcontainers-go/wait"
	"gitlab.com/gitlab-org/api/client-go"

	"github.com/moby/moby/api/types/container"
)

// These E2E tests are opt-in because they start real GitLab CE and Vault
// containers, create a GitLab project access token, and write it to Vault KVv2.
//
// Run:
//
//	go test -tags=e2e ./e2e -timeout 30m
//
// Optional environment variables:
//
//	TOKEN_TUMBLER_E2E_GITLAB_IMAGE  default: gitlab/gitlab-ce:17.11.0-ce.0
//	TOKEN_TUMBLER_E2E_VAULT_IMAGE   default: hashicorp/vault:1.13.0
func TestE2E_GitLabProjectTokenLifecycleWithVault(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	t.Cleanup(cancel)

	runID := fmt.Sprintf("token-tumbler-e2e-%d", time.Now().UnixNano())
	vaultMount := "kv"
	vaultPath := "token-tumbler/e2e/" + runID
	vaultKey := "gitlab_token"

	vaultAddr, roleID, secretID := startVaultContainer(t, ctx, vaultMount)
	t.Setenv("VAULT_ADDR", vaultAddr)
	t.Setenv("APPROLE_ID", roleID)
	t.Setenv("APPROLE_SECRET", secretID)

	gitlabClient, gitlabProjectID, gitlabProjectPath := startGitLabContainerAndCreateProject(t, ctx, runID)

	freshEntry := &repository.Repository{
		RepoName:          gitlab.Ptr(gitlabProjectPath),
		Name:              runID + "-fresh",
		Permissions:       []string{"api"},
		RotationThreshold: &repository.Duration{Duration: 24 * time.Hour},
		GracePeriod:       &repository.Duration{Duration: -1 * time.Hour},
		Lifetime:          repository.Duration{Duration: 72 * time.Hour},
	}

	expectedFreshExpiry := time.Now().Add(freshEntry.Lifetime.ToDuration()).Format(time.DateOnly)
	createdToken, err := project.CreateNewProjectToken(gitlabClient, gitlabProjectID, freshEntry, "tt-e2e")
	require.NoError(t, err)
	require.NotNil(t, createdToken)
	require.NotZero(t, createdToken.ID)
	require.NotEmpty(t, createdToken.Token)
	require.NotEmpty(t, createdToken.Name)
	require.NotNil(t, createdToken.ExpiresAt)
	assert.Equal(t, expectedFreshExpiry, createdToken.ExpiresAt.String())

	nameMatches, err := freshEntry.ParseTokenName("tt-e2e", createdToken.Name)
	require.NoError(t, err)
	assert.True(t, nameMatches)

	needsRenewal, err := freshEntry.ShouldBeRenewed(createdToken)
	require.NoError(t, err)
	assert.False(t, needsRenewal)

	allNeedRenewal, err := project.CheckProjectTokensForRenewal([]*gitlab.ProjectAccessToken{createdToken}, freshEntry)
	require.NoError(t, err)
	assert.False(t, allNeedRenewal)

	projectTokens, err := project.GatherProjectTokenInfo(gitlabClient, gitlabProjectID)
	require.NoError(t, err)
	assert.Len(t, filterProjectTokensByEntry(t, projectTokens, freshEntry, "tt-e2e"), 1)

	t.Cleanup(func() {
		_, _ = gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(gitlabProjectID, createdToken.ID)
	})

	vaultSecret := &secrets.VaultSecret{
		Path:      vaultPath,
		Key:       vaultKey,
		MountPath: vaultMount,
	}
	t.Cleanup(func() {
		if vaultSecret.Client != nil {
			_ = vaultSecret.Client.KVv2(vaultMount).DeleteMetadata(context.Background(), vaultPath)
		}
	})

	require.NoError(t, vaultSecret.Write(ctx, createdToken.Token))

	readBack, err := (&secrets.VaultSecret{
		Path:      vaultPath,
		Key:       vaultKey,
		MountPath: vaultMount,
	}).Read(ctx)

	require.NoError(t, err)
	assert.Equal(t, createdToken.Token, readBack)

	rotateEntry := &repository.Repository{
		RepoName:          gitlab.Ptr(gitlabProjectPath),
		Name:              runID + "-rotate",
		Permissions:       []string{"api"},
		RotationThreshold: &repository.Duration{Duration: 7 * 24 * time.Hour},
		GracePeriod:       &repository.Duration{Duration: -1 * time.Hour},
		Lifetime:          repository.Duration{Duration: 24 * time.Hour},
	}

	expectedRotationExpiry := time.Now().Add(rotateEntry.Lifetime.ToDuration()).Format(time.DateOnly)
	oldRotationToken, err := project.CreateNewProjectToken(gitlabClient, gitlabProjectID, rotateEntry, "tt-e2e")
	require.NoError(t, err)
	require.NotNil(t, oldRotationToken)
	require.NotZero(t, oldRotationToken.ID)
	require.NotEmpty(t, oldRotationToken.Token)
	require.NotNil(t, oldRotationToken.ExpiresAt)
	assert.Equal(t, expectedRotationExpiry, oldRotationToken.ExpiresAt.String())
	t.Cleanup(func() {
		_, _ = gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(gitlabProjectID, oldRotationToken.ID)
	})

	needsRenewal, err = rotateEntry.ShouldBeRenewed(oldRotationToken)
	require.NoError(t, err)
	assert.True(t, needsRenewal)

	allNeedRenewal, err = project.CheckProjectTokensForRenewal([]*gitlab.ProjectAccessToken{oldRotationToken}, rotateEntry)
	require.NoError(t, err)
	assert.True(t, allNeedRenewal)

	time.Sleep(2 * time.Second)
	renewedRotationToken, err := project.RenewProjectAccessToken(gitlabClient, gitlabProjectID, rotateEntry, "tt-e2e")
	require.NoError(t, err)
	require.NotNil(t, renewedRotationToken)
	require.NotZero(t, renewedRotationToken.ID)
	require.NotEqual(t, oldRotationToken.ID, renewedRotationToken.ID)
	require.NotEmpty(t, renewedRotationToken.Token)
	t.Cleanup(func() {
		_, _ = gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(gitlabProjectID, renewedRotationToken.ID)
	})

	projectTokens, err = project.GatherProjectTokenInfo(gitlabClient, gitlabProjectID)
	require.NoError(t, err)
	rotationTokens := filterProjectTokensByEntry(t, projectTokens, rotateEntry, "tt-e2e")
	require.Len(t, rotationTokens, 2)

	require.NoError(t, project.DeleteProjectTokens(gitlabClient, rotateEntry, "tt-e2e"))

	projectTokens, err = project.GatherProjectTokenInfo(gitlabClient, gitlabProjectID)
	require.NoError(t, err)
	remainingRotationTokens := filterProjectTokensByEntry(t, projectTokens, rotateEntry, "tt-e2e")
	activeRemainingRotationTokens := activeProjectTokens(remainingRotationTokens)
	require.Len(t, activeRemainingRotationTokens, 1)
	assert.Equal(t, renewedRotationToken.ID, activeRemainingRotationTokens[0].ID)
	assertProjectTokenRevoked(t, remainingRotationTokens, oldRotationToken.ID)

	t.Logf("verified GitLab token lifetime, renewal threshold, rotation cleanup, and Vault secret at %s/%s[%s]", vaultMount, vaultPath, vaultKey)
}

func filterProjectTokensByEntry(t *testing.T, tokens []*gitlab.ProjectAccessToken, entry *repository.Repository, prefix string) []*gitlab.ProjectAccessToken {
	t.Helper()

	var matching []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		matches, err := entry.ParseTokenName(prefix, token.Name)
		if err != nil {
			continue
		}
		if matches {
			matching = append(matching, token)
		}
	}

	return matching
}

func activeProjectTokens(tokens []*gitlab.ProjectAccessToken) []*gitlab.ProjectAccessToken {
	var active []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		if !token.Revoked && token.Active {
			active = append(active, token)
		}
	}
	return active
}

func assertProjectTokenRevoked(t *testing.T, tokens []*gitlab.ProjectAccessToken, id int64) {
	t.Helper()

	for _, token := range tokens {
		if token.ID == id {
			assert.True(t, token.Revoked)
			assert.False(t, token.Active)
			return
		}
	}

	t.Fatalf("expected token %d to be present as revoked in GitLab token list", id)
}

func startVaultContainer(t *testing.T, ctx context.Context, mount string) (addr string, roleID string, secretID string) {
	t.Helper()
	const rootToken = "token-tumbler-root-token"

	vaultContainer, err := tcvault.Run(ctx, getenvDefault("TOKEN_TUMBLER_E2E_VAULT_IMAGE", "hashicorp/vault:1.13.0"), tcvault.WithToken(rootToken))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(vaultContainer)) })

	addr, err = vaultContainer.HttpHostAddress(ctx)
	require.NoError(t, err)

	client, err := vaultapi.NewClient(&vaultapi.Config{Address: addr})
	require.NoError(t, err)
	client.SetToken(rootToken)

	enableVaultMount(t, ctx, client, "sys/auth/approle", map[string]interface{}{"type": "approle"})
	enableVaultMount(t, ctx, client, "sys/mounts/"+mount, map[string]interface{}{
		"type":    "kv",
		"options": map[string]string{"version": "2"},
	})

	policy := fmt.Sprintf(`path %q { capabilities = ["create", "update", "read"] }
path %q { capabilities = ["delete", "read"] }
`, mount+"/data/token-tumbler/e2e/*", mount+"/metadata/token-tumbler/e2e/*")
	require.NoError(t, client.Sys().PutPolicyWithContext(ctx, "token-tumbler-e2e", policy))

	_, err = client.Logical().WriteWithContext(ctx, "auth/approle/role/token-tumbler-e2e", map[string]interface{}{
		"token_policies": "token-tumbler-e2e",
		"token_ttl":      "1h",
		"token_max_ttl":  "4h",
	})
	require.NoError(t, err)

	roleSecret, err := client.Logical().ReadWithContext(ctx, "auth/approle/role/token-tumbler-e2e/role-id")
	require.NoError(t, err)
	require.NotNil(t, roleSecret)
	roleID, ok := roleSecret.Data["role_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, roleID)

	secret, err := client.Logical().WriteWithContext(ctx, "auth/approle/role/token-tumbler-e2e/secret-id", nil)
	require.NoError(t, err)
	require.NotNil(t, secret)
	secretID, ok = secret.Data["secret_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, secretID)

	return addr, roleID, secretID
}

func enableVaultMount(t *testing.T, ctx context.Context, client *vaultapi.Client, path string, data map[string]interface{}) {
	t.Helper()
	_, err := client.Logical().WriteWithContext(ctx, path, data)
	if err != nil && !strings.Contains(err.Error(), "path is already in use") {
		require.NoError(t, err)
	}
}

func startGitLabContainerAndCreateProject(t *testing.T, ctx context.Context, runID string) (*gitlab.Client, int64, string) {
	t.Helper()
	rootToken := "tokenchaserrootpat123456"
	rootPassword := "Zx9$Qv2!Lm7#Rp4%Tn8@Ys6"

	gitlabContainer, err := testcontainers.Run(ctx, getenvDefault("TOKEN_TUMBLER_E2E_GITLAB_IMAGE", "gitlab/gitlab-ce:17.11.0-ce.0"),
		testcontainers.WithEnv(map[string]string{
			"GITLAB_OMNIBUS_CONFIG": "external_url 'http://localhost'; gitlab_rails['initial_root_password'] = '" + rootPassword + "';",
		}),
		testcontainers.WithExposedPorts("80/tcp"),
		testcontainers.WithHostConfigModifier(func(hc *container.HostConfig) {
			hc.ShmSize = 256 * 1024 * 1024
		}),
		testcontainers.WithWaitStrategyAndDeadline(20*time.Minute,
			wait.ForLog("gitlab Reconfigured!").WithStartupTimeout(20*time.Minute),
		),
	)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, testcontainers.TerminateContainer(gitlabContainer)) })

	createRootPAT(t, ctx, gitlabContainer, rootToken)

	gitlabURL, err := gitlabContainer.PortEndpoint(ctx, "80/tcp", "http")
	require.NoError(t, err)
	gitlabClient, err := gitlab.NewClient(rootToken, gitlab.WithBaseURL(gitlabURL+"/api/v4"))
	require.NoError(t, err)
	waitForGitLabAPI(t, ctx, gitlabClient)

	path := sanitizeGitLabProjectPath(runID)
	var createdProject *gitlab.Project
	retryUntil(t, ctx, 5*time.Minute, 5*time.Second, func() error {
		var createErr error
		createdProject, _, createErr = gitlabClient.Projects.CreateProject(&gitlab.CreateProjectOptions{
			Name:       gitlab.Ptr(path),
			Path:       gitlab.Ptr(path),
			Visibility: gitlab.Ptr(gitlab.PrivateVisibility),
		})
		return createErr
	})
	require.NotNil(t, createdProject)
	t.Cleanup(func() { _, _ = gitlabClient.Projects.DeleteProject(createdProject.ID, nil) })

	projectPath := createdProject.PathWithNamespace
	if strings.TrimSpace(projectPath) == "" {
		projectPath = path
	}
	return gitlabClient, createdProject.ID, projectPath
}

func createRootPAT(t *testing.T, ctx context.Context, container testcontainers.Container, token string) {
	t.Helper()
	ruby := fmt.Sprintf(`user = User.find_by_username('root'); existing = user.personal_access_tokens.find_by(name: 'token-tumbler-e2e-root'); existing&.destroy!; pat = user.personal_access_tokens.create!(name: 'token-tumbler-e2e-root', scopes: ['api'], expires_at: 1.day.from_now); pat.set_token('%s'); pat.save!`, token)
	retryUntil(t, ctx, 5*time.Minute, 10*time.Second, func() error {
		code, output, err := container.Exec(ctx, []string{"bash", "-lc", "gitlab-rails runner " + shellQuote(ruby)})
		contents, _ := io.ReadAll(output)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("gitlab-rails runner failed with code %d: %s", code, string(contents))
		}
		return nil
	})
}

func waitForGitLabAPI(t *testing.T, ctx context.Context, client *gitlab.Client) {
	t.Helper()
	retryUntil(t, ctx, 5*time.Minute, 5*time.Second, func() error {
		_, _, err := client.Version.GetVersion()
		return err
	})
}

func retryUntil(t *testing.T, ctx context.Context, timeout time.Duration, interval time.Duration, fn func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := fn(); err == nil {
			return
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			require.NoError(t, ctx.Err())
		case <-time.After(interval):
		}
	}
	require.NoError(t, lastErr)
}

func sanitizeGitLabProjectPath(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func getenvDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
