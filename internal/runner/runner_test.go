package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

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
	client := newTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
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
	client := newTestGitlabClient(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	NewRunner(client, &repository.Config{Prefix: "tt"}, logger.GetLogger()).ProcessRepository(ctx, repo, 0)

	assert.Zero(t, requestCount)
}

func newTestGitlabClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
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
