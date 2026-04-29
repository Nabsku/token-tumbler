package project

import (
	"fmt"
	"github.com/nabsku/token-chaser/internal/types/repository"
	"gitlab.com/gitlab-org/api/client-go"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func projectAccessToken(id int, createdAt time.Time) *gitlab.ProjectAccessToken {
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
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects":
			assert.Equal(t, repoName, r.URL.Query().Get("search"))
			_, _ = w.Write([]byte(`[{"id":42,"name":"service"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-service-only","active":true,"created_at":%q},
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

	err := DeleteProjectTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteProjectTokens_ShouldDeleteOnlyOlderMatchingTokensAfterGracePeriod(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":42,"name":"service"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-service-oldest","active":true,"created_at":%q},
				{"id":2,"name":"tc-service-old","active":true,"created_at":%q},
				{"id":3,"name":"tc-service-newest","active":true,"created_at":%q},
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

	err := DeleteProjectTokens(client, repo, "tc")

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
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":42,"name":"service"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-service-old","active":true,"created_at":%q},
				{"id":2,"name":"tc-service-newest","active":true,"created_at":%q}
			]`, time.Now().Add(-72*time.Hour).Format(time.RFC3339), time.Now().Add(-48*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			http.Error(w, "revoke failed", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(client, repo, "tc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting token")
}

func TestDeleteProjectTokens_ShouldIgnoreRevokedAndInactiveTokens(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":42,"name":"service"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-service-revoked","active":false,"revoked":true,"created_at":%q},
				{"id":2,"name":"tc-service-inactive","active":false,"created_at":%q},
				{"id":3,"name":"tc-service-newest","active":true,"created_at":%q}
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

	err := DeleteProjectTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteProjectTokens_ShouldNotDeleteWhenGracePeriodHasNotPassed(t *testing.T) {
	repoName := "service"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects":
			_, _ = w.Write([]byte(`[{"id":42,"name":"service"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-service-old","active":true,"created_at":%q},
				{"id":2,"name":"tc-service-newest","active":true,"created_at":%q}
			]`, time.Now().Add(-20*time.Hour).Format(time.RFC3339), time.Now().Add(-10*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteProjectTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteProjectTokens_ShouldReturnErrorWhenProjectIsMissing(t *testing.T) {
	repoName := "missing"
	repo := &repository.Repository{Name: "service", RepoName: &repoName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	client := newProjectTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v4/projects", r.URL.Path)
		_, _ = w.Write([]byte(`[]`))
	})

	err := DeleteProjectTokens(client, repo, "tc")

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no projects found"))
}
