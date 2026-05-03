package project

import (
	"context"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestCheckProjectTokenDeletion_ShouldDelete(t *testing.T) {
	repo := &repository.Repository{
		GracePeriod: &repository.Duration{Duration: 3 * 24 * time.Hour},
	}
	oldToken := projectAccessToken(1, time.Now().Add(-5*24*time.Hour))
	newestToken := projectAccessToken(2, time.Now().Add(-4*24*time.Hour))

	shouldDelete := checkProjectTokenDeletion(repo, oldToken, newestToken)
	assert.True(t, shouldDelete)
}

func TestCheckProjectTokenDeletion_ShouldNotDelete(t *testing.T) {
	repo := &repository.Repository{
		GracePeriod: &repository.Duration{Duration: 3 * 24 * time.Hour},
	}
	oldToken := projectAccessToken(1, time.Now().Add(-5*24*time.Hour))

	newestTokenMinusTwo := projectAccessToken(2, time.Now().AddDate(0, 0, -2))
	shouldNotDeleteMinusTwo := checkProjectTokenDeletion(repo, oldToken, newestTokenMinusTwo)

	newestTokenToday := projectAccessToken(2, time.Now())
	shouldNotDeleteToday := checkProjectTokenDeletion(repo, oldToken, newestTokenToday)

	newestToken := projectAccessToken(2, time.Now().Add(-4*24*time.Hour))
	shouldNotDeleteNewestToken := checkProjectTokenDeletion(repo, newestToken, newestToken)

	assert.False(t, shouldNotDeleteMinusTwo)
	assert.False(t, shouldNotDeleteToday)
	assert.False(t, shouldNotDeleteNewestToken)
}

func TestCheckProjectTokenDeletion_ShouldNotDeleteTokenWithUnknownCreationDate(t *testing.T) {
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: time.Hour}}
	token := &gitlab.ProjectAccessToken{}
	token.ID = 1
	newestToken := projectAccessToken(2, time.Now().Add(-2*time.Hour))

	shouldDelete := checkProjectTokenDeletion(repo, token, newestToken)

	assert.False(t, shouldDelete)
}

func TestCheckProjectTokenDeletionAt_GracePeriodBoundary(t *testing.T) {
	createdAt := time.Date(2026, time.January, 13, 12, 0, 0, 0, time.UTC)
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	oldToken := projectAccessToken(1, createdAt.Add(-time.Hour))
	preserveToken := projectAccessToken(2, createdAt)

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "before grace boundary", now: createdAt.Add(24*time.Hour - time.Nanosecond), want: false},
		{name: "at grace boundary", now: createdAt.Add(24 * time.Hour), want: false},
		{name: "after grace boundary", now: createdAt.Add(24*time.Hour + time.Nanosecond), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkProjectTokenDeletionAt(repo, oldToken, preserveToken, tt.now)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckProjectTokenDeletionAt_ZeroGracePeriodBoundary(t *testing.T) {
	createdAt := time.Date(2026, time.January, 13, 12, 0, 0, 0, time.UTC)
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 0}}
	oldToken := projectAccessToken(1, createdAt.Add(-time.Hour))
	preserveToken := projectAccessToken(2, createdAt)

	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{name: "at creation boundary", now: createdAt, want: false},
		{name: "after creation boundary", now: createdAt.Add(time.Nanosecond), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkProjectTokenDeletionAt(repo, oldToken, preserveToken, tt.now)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvePreserveToken_ProjectPolicy(t *testing.T) {
	older := projectAccessToken(1, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	newest := projectAccessToken(2, time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC))
	tokens := []*gitlab.ProjectAccessToken{older, newest}

	tests := []struct {
		name         string
		vaultTokenID int64
		wantID       int64
	}{
		{name: "vault token id matches newest", vaultTokenID: newest.ID, wantID: newest.ID},
		{name: "vault token id missing falls back to newest", vaultTokenID: 99, wantID: newest.ID},
		{name: "no vault token id falls back to newest", vaultTokenID: 0, wantID: newest.ID},
		{name: "vault token id matches older persisted token", vaultTokenID: older.ID, wantID: older.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePreserveToken(tokens, tt.vaultTokenID, zap.NewNop(), "service")

			require.NotNil(t, got)
			assert.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestCheckProjectTokenDeletionAt_DoesNotDeleteNewerOrphanThanVaultPreservedToken(t *testing.T) {
	now := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	preserveToken := projectAccessToken(1, now.Add(-72*time.Hour))
	newerOrphan := projectAccessToken(2, now.Add(-48*time.Hour))
	olderToken := projectAccessToken(3, now.Add(-96*time.Hour))

	assert.False(t, checkProjectTokenDeletionAt(repo, newerOrphan, preserveToken, now))
	assert.True(t, checkProjectTokenDeletionAt(repo, olderToken, preserveToken, now))
}

func projectAccessToken(id int64, createdAt time.Time) *gitlab.ProjectAccessToken {
	token := &gitlab.ProjectAccessToken{}
	token.ID = id
	token.CreatedAt = &createdAt
	token.Active = true
	return token
}

func TestDeleteProjectTokens_ShouldNotDeleteWhenOneMatchingTokenExists(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tt-service-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"foreign-token","active":true,"created_at":%q}
			]`, time.Now().Add(-72*time.Hour).Format(time.RFC3339), time.Now().Add(-72*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(context.Background(), client, repo, "tt", 0)

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteProjectTokens_ShouldDeleteOnlyOlderMatchingTokensAfterGracePeriod(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tt-service-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"tt-service-2026-01-02T00:00:00Z","active":true,"created_at":%q},
				{"id":3,"name":"tt-service-2026-01-03T00:00:00Z","active":true,"created_at":%q},
				{"id":4,"name":"foreign-token","active":true,"created_at":%q}
			]`,
				time.Now().Add(-72*time.Hour).Format(time.RFC3339),
				time.Now().Add(-60*time.Hour).Format(time.RFC3339),
				time.Now().Add(-48*time.Hour).Format(time.RFC3339),
				time.Now().Add(-96*time.Hour).Format(time.RFC3339),
			)))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(context.Background(), client, repo, "tt", 0)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"/api/v4/projects/42/access_tokens/1",
		"/api/v4/projects/42/access_tokens/2",
	}, deleteCalls)
}

func TestDeleteProjectTokens_ShouldReturnRevokeErrors(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tt-service-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"tt-service-2026-01-02T00:00:00Z","active":true,"created_at":%q}
			]`, time.Now().Add(-72*time.Hour).Format(time.RFC3339), time.Now().Add(-48*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			http.Error(w, "revoke failed", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(context.Background(), client, repo, "tt", 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting token")
}

func TestDeleteProjectTokens_ShouldIgnoreRevokedAndInactiveTokens(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tt-service-2026-01-01T00:00:00Z","active":false,"revoked":true,"created_at":%q},
				{"id":2,"name":"tt-service-2026-01-02T00:00:00Z","active":false,"created_at":%q},
				{"id":3,"name":"tt-service-2026-01-03T00:00:00Z","active":true,"created_at":%q}
			]`,
				time.Now().Add(-96*time.Hour).Format(time.RFC3339),
				time.Now().Add(-84*time.Hour).Format(time.RFC3339),
				time.Now().Add(-72*time.Hour).Format(time.RFC3339),
			)))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(context.Background(), client, repo, "tt", 0)

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteProjectTokens_ShouldNotDeleteWhenGracePeriodHasNotPassed(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/service":
			_, _ = w.Write([]byte(`{"id":42,"name":"service"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tt-service-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"tt-service-2026-01-02T00:00:00Z","active":true,"created_at":%q}
			]`, time.Now().Add(-20*time.Hour).Format(time.RFC3339), time.Now().Add(-10*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(context.Background(), client, repo, "tt", 0)

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteProjectTokens_ShouldReturnErrorWhenProjectIsMissing(t *testing.T) {
	repoName := "missing"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/projects/missing", r.URL.Path)
		http.Error(w, "not found", http.StatusNotFound)
	})

	err := DeleteProjectTokens(context.Background(), client, repo, "tt", 0)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no projects found"))
}
