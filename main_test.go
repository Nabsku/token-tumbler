package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/secrets"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestCheckEnvVars(t *testing.T) {
	t.Run("returns nil when all variables exist", func(t *testing.T) {
		t.Setenv("TOKEN_TUMBLER_MAIN_TEST_A", "a")
		t.Setenv("TOKEN_TUMBLER_MAIN_TEST_B", "b")

		err := checkEnvVars("TOKEN_TUMBLER_MAIN_TEST_A", "TOKEN_TUMBLER_MAIN_TEST_B")

		require.NoError(t, err)
	})

	t.Run("returns joined missing variables", func(t *testing.T) {
		err := checkEnvVars("TOKEN_TUMBLER_MAIN_TEST_MISSING_A", "TOKEN_TUMBLER_MAIN_TEST_MISSING_B")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_TUMBLER_MAIN_TEST_MISSING_A, TOKEN_TUMBLER_MAIN_TEST_MISSING_B")
	})

	t.Run("treats empty variables as missing", func(t *testing.T) {
		t.Setenv("TOKEN_TUMBLER_MAIN_TEST_EMPTY", "")

		err := checkEnvVars("TOKEN_TUMBLER_MAIN_TEST_EMPTY")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_TUMBLER_MAIN_TEST_EMPTY")
	})
}

func TestPollIntervalFromEnv(t *testing.T) {
	t.Run("defaults to production interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "")

		interval, err := pollIntervalFromEnv()

		require.NoError(t, err)
		assert.Equal(t, defaultPollInterval, interval)
	})

	t.Run("uses environment interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "15m")

		interval, err := pollIntervalFromEnv()

		require.NoError(t, err)
		assert.Equal(t, 15*time.Minute, interval)
	})

	t.Run("rejects invalid interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "soon")

		interval, err := pollIntervalFromEnv()

		require.Error(t, err)
		assert.Zero(t, interval)
	})

	t.Run("rejects non-positive interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "0s")

		interval, err := pollIntervalFromEnv()

		require.Error(t, err)
		assert.Zero(t, interval)
	})
}

func TestNewClient(t *testing.T) {
	t.Setenv("GITLAB_URL", "https://gitlab.example.com")
	t.Setenv("GITLAB_TOKEN", "token")

	client, err := NewClient()

	require.NoError(t, err)
	require.NotNil(t, client)
}

func TestReadConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("config.yaml", []byte(`prefix: tt
repositories:
  - repoName: service
    name: token
    permissions:
      - api
    rotationThreshold: 1d
    gracePeriod: 1d
    lifetime: 2d
    secretStore: none
`), 0o600))

	config, err := readConfig()

	require.NoError(t, err)
	require.NotNil(t, config)
	assert.NotEmpty(t, config.Prefix)
	assert.NotEmpty(t, config.Repos)
	for _, repo := range config.Repos {
		assert.NotEmpty(t, repo.Name)
		assert.NotZero(t, repo.Lifetime.ToDuration())
		if repo.RotationThreshold != nil {
			assert.NoError(t, (&repository.Repository{Lifetime: repo.Lifetime, RotationThreshold: repo.RotationThreshold}).CheckKeyRotationAndTokenAge())
		}
	}
}

func TestCheckedInConfig_ShouldValidate(t *testing.T) {
	config, err := readConfig()

	require.NoError(t, err)
	require.NotNil(t, config)
}

func TestMatchingProjectTokens_ShouldSkipForeignTokens(t *testing.T) {
	repo := &repository.Repository{RepoName: gitlab.Ptr("service"), Name: "service"}
	tokens := []*gitlab.ProjectAccessToken{
		projectTokenNamed("foreign-token"),
		revokedProjectTokenNamed("tt-service-revoked"),
		inactiveProjectTokenNamed("tt-service-inactive"),
		projectTokenNamed("tt-service-2026-01-01T00:00:00Z"),
	}

	got := matchingProjectTokens(tokens, repo, "tt", 0)

	require.Len(t, got, 1)
	assert.Equal(t, "tt-service-2026-01-01T00:00:00Z", got[0].Name)
}

func TestMatchingGroupTokens_ShouldSkipForeignTokens(t *testing.T) {
	repo := &repository.Repository{GroupName: gitlab.Ptr("platform"), Name: "platform"}
	tokens := []*gitlab.GroupAccessToken{
		groupTokenNamed("foreign-token"),
		revokedGroupTokenNamed("tt-platform-revoked"),
		inactiveGroupTokenNamed("tt-platform-inactive"),
		groupTokenNamed("tt-platform-2026-01-01T00:00:00Z"),
	}

	got := matchingGroupTokens(tokens, repo, "tt", 0)

	require.Len(t, got, 1)
	assert.Equal(t, "tt-platform-2026-01-01T00:00:00Z", got[0].Name)
}

func projectTokenNamed(name string) *gitlab.ProjectAccessToken {
	token := &gitlab.ProjectAccessToken{}
	token.Name = name
	token.Active = true
	return token
}

func revokedProjectTokenNamed(name string) *gitlab.ProjectAccessToken {
	token := projectTokenNamed(name)
	token.Revoked = true
	token.Active = false
	return token
}

func inactiveProjectTokenNamed(name string) *gitlab.ProjectAccessToken {
	token := projectTokenNamed(name)
	token.Active = false
	return token
}

func groupTokenNamed(name string) *gitlab.GroupAccessToken {
	token := &gitlab.GroupAccessToken{}
	token.Name = name
	token.Active = true
	return token
}

func revokedGroupTokenNamed(name string) *gitlab.GroupAccessToken {
	token := groupTokenNamed(name)
	token.Revoked = true
	token.Active = false
	return token
}

func inactiveGroupTokenNamed(name string) *gitlab.GroupAccessToken {
	token := groupTokenNamed(name)
	token.Active = false
	return token
}

func TestSecretStoreForToken(t *testing.T) {
	t.Run("allows explicit none", func(t *testing.T) {
		secret, err := secrets.ForRepository(&repository.Repository{SecretStore: "none"})

		require.NoError(t, err)
		assert.Nil(t, secret)
	})

	t.Run("rejects unsupported store", func(t *testing.T) {
		secret, err := secrets.ForRepository(&repository.Repository{SecretStore: "file"})

		require.Error(t, err)
		assert.Nil(t, secret)
	})
}

func TestProcessRepository_ShouldNotDeleteOldProjectTokenWhenVaultWriteFails(t *testing.T) {
	t.Setenv("APPROLE_ID", "")
	t.Setenv("APPROLE_SECRET", "")
	repoName := "service"
	vaultPath := "teams/service"
	vaultKey := "gitlab_token"
	vaultMount := "kv"
	repo := repository.Repository{
		RepoName:          &repoName,
		Name:              "service",
		Permissions:       []string{"api"},
		RotationThreshold: &repository.Duration{Duration: 24 * time.Hour},
		GracePeriod:       &repository.Duration{Duration: time.Hour},
		Lifetime:          repository.Duration{Duration: 72 * time.Hour},
		SecretStore:       "vault",
		VaultPath:         &vaultPath,
		VaultKey:          &vaultKey,
		Mount:             &vaultMount,
	}
	deleteCalls := make([]string, 0)
	client := newMainTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(`{"id":99,"name":"tt-service-new","token":"new-secret","active":true}`))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	NewRunner(client, &repository.Config{Prefix: "tt"}, logger.GetLogger()).ProcessRepository(context.Background(), repo, 0)

	assert.Empty(t, deleteCalls)
}

func TestProcessRepository_ShouldNotDeleteOldGroupTokenWhenVaultWriteFails(t *testing.T) {
	t.Setenv("APPROLE_ID", "")
	t.Setenv("APPROLE_SECRET", "")
	groupName := "platform"
	vaultPath := "teams/platform"
	vaultKey := "gitlab_token"
	vaultMount := "kv"
	repo := repository.Repository{
		GroupName:         &groupName,
		Name:              "platform",
		Permissions:       []string{"api"},
		RotationThreshold: &repository.Duration{Duration: 24 * time.Hour},
		GracePeriod:       &repository.Duration{Duration: time.Hour},
		Lifetime:          repository.Duration{Duration: 72 * time.Hour},
		SecretStore:       "vault",
		VaultPath:         &vaultPath,
		VaultKey:          &vaultKey,
		Mount:             &vaultMount,
	}
	deleteCalls := make([]string, 0)
	client := newMainTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/platform":
			_, _ = w.Write([]byte(`{"id":42,"name":"platform"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(`{"id":99,"name":"tt-platform-new","token":"new-secret","active":true}`))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	NewRunner(client, &repository.Config{Prefix: "tt"}, logger.GetLogger()).ProcessRepository(context.Background(), repo, 0)

	assert.Empty(t, deleteCalls)
}

func TestProcessRepository_ShouldAttemptProjectDeletionAndStopOnRevokeFailure(t *testing.T) {
	repoName := "service"
	repo := repository.Repository{
		RepoName:          &repoName,
		Name:              "service",
		Permissions:       []string{"api"},
		RotationThreshold: &repository.Duration{Duration: 24 * time.Hour},
		GracePeriod:       &repository.Duration{Duration: time.Hour},
		Lifetime:          repository.Duration{Duration: 72 * time.Hour},
		SecretStore:       "none",
	}
	deleteCalls := make([]string, 0)
	client := newMainTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tt-service-old","active":true,"created_at":%q,"expires_at":%q},
				{"id":2,"name":"tt-service-newest","active":true,"created_at":%q,"expires_at":%q}
			]`,
				time.Now().Add(-72*time.Hour).Format(time.RFC3339), time.Now().Add(48*time.Hour).Format(time.DateOnly),
				time.Now().Add(-48*time.Hour).Format(time.RFC3339), time.Now().Add(48*time.Hour).Format(time.DateOnly),
			)))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			http.Error(w, "revoke failed", http.StatusBadRequest)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	NewRunner(client, &repository.Config{Prefix: "tt"}, logger.GetLogger()).ProcessRepository(context.Background(), repo, 0)

	require.NotEmpty(t, deleteCalls)
	assert.ElementsMatch(t, []string{"/api/v4/projects/42/access_tokens/1"}, uniqueStrings(deleteCalls))
}

func TestProcessRepository_ShouldSkipWorkWhenContextIsCanceled(t *testing.T) {
	repoName := "service"
	repo := repository.Repository{
		RepoName:          &repoName,
		Name:              "service",
		Permissions:       []string{"api"},
		RotationThreshold: &repository.Duration{Duration: 24 * time.Hour},
		GracePeriod:       &repository.Duration{Duration: time.Hour},
		Lifetime:          repository.Duration{Duration: 72 * time.Hour},
		SecretStore:       "none",
	}
	requestCount := 0
	client := newMainTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	NewRunner(client, &repository.Config{Prefix: "tt"}, logger.GetLogger()).ProcessRepository(ctx, repo, 0)

	assert.Zero(t, requestCount)
}

func newMainTestGitlabClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := gitlab.NewClient("token", gitlab.WithBaseURL(server.URL+"/api/v4"))
	require.NoError(t, err)
	return client
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

type mockSecretStore struct {
	readValue       string
	readErr         error
	writeCalls      []string
	writeErr        error
	writeErrOnCall  int // 0 = all calls, N = only on Nth call (1-based)
	writeCallCount  int
	writeMetaErr    error
	readMetaValue   secrets.TokenMetadata
	readMetaErr     error
}

func (m *mockSecretStore) Read(_ context.Context) (string, error) {
	return m.readValue, m.readErr
}

func (m *mockSecretStore) Write(_ context.Context, value string) error {
	m.writeCallCount++
	m.writeCalls = append(m.writeCalls, value)
	if m.writeErr != nil && (m.writeErrOnCall == 0 || m.writeErrOnCall == m.writeCallCount) {
		return m.writeErr
	}
	return nil
}

func (m *mockSecretStore) InitClient(_ context.Context) error {
	return nil
}

func (m *mockSecretStore) ReadMetadata(_ context.Context) (secrets.TokenMetadata, error) {
	return m.readMetaValue, m.readMetaErr
}

func (m *mockSecretStore) WriteMetadata(_ context.Context, _ secrets.TokenMetadata) error {
	return m.writeMetaErr
}

func TestPersistToken(t *testing.T) {
	t.Run("returns nil when secret store is nil", func(t *testing.T) {
		err := persistToken(context.Background(), nil, nil, "token", secrets.TokenMetadata{})
		require.NoError(t, err)
	})

	t.Run("returns error when write fails", func(t *testing.T) {
		mock := &mockSecretStore{writeErr: fmt.Errorf("write failed")}
		err := persistToken(context.Background(), nil, mock, "new-token", secrets.TokenMetadata{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "writing token")
		assert.Equal(t, []string{"new-token"}, mock.writeCalls)
	})

	t.Run("returns error when metadata write fails without restoring if read failed", func(t *testing.T) {
		mock := &mockSecretStore{
			readErr:      fmt.Errorf("not found"),
			writeMetaErr: fmt.Errorf("metadata failed"),
		}
		err := persistToken(context.Background(), nil, mock, "new-token", secrets.TokenMetadata{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "writing metadata")
		assert.Equal(t, []string{"new-token"}, mock.writeCalls)
	})

	t.Run("restores old value when metadata write fails", func(t *testing.T) {
		mock := &mockSecretStore{
			readValue:    "old-token",
			writeMetaErr: fmt.Errorf("metadata failed"),
		}
		err := persistToken(context.Background(), nil, mock, "new-token", secrets.TokenMetadata{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "writing metadata")
		assert.Equal(t, []string{"new-token", "old-token"}, mock.writeCalls)
	})

	t.Run("returns combined error when restore fails", func(t *testing.T) {
		mock := &mockSecretStore{
			readValue:      "old-token",
			writeMetaErr:   fmt.Errorf("metadata failed"),
			writeErr:       fmt.Errorf("restore failed"),
			writeErrOnCall: 2, // fail on the restore write only
		}
		err := persistToken(context.Background(), nil, mock, "new-token", secrets.TokenMetadata{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unable to restore previous token")
		assert.Equal(t, []string{"new-token", "old-token"}, mock.writeCalls)
	})

	t.Run("succeeds when both writes succeed", func(t *testing.T) {
		mock := &mockSecretStore{readValue: "old-token"}
		err := persistToken(context.Background(), nil, mock, "new-token", secrets.TokenMetadata{TokenID: 1})
		require.NoError(t, err)
		assert.Equal(t, []string{"new-token"}, mock.writeCalls)
	})
}
