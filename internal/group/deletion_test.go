package group

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nabsku/token-chaser/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestCheckGroupTokenDeletion_ShouldDelete(t *testing.T) {
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 3 * 24 * time.Hour}}
	oldToken := groupAccessToken(1, time.Now().Add(-5*24*time.Hour))
	newestToken := groupAccessToken(2, time.Now().Add(-4*24*time.Hour))

	shouldDelete := checkGroupTokenDeletion(repo, oldToken, newestToken)

	assert.True(t, shouldDelete)
}

func TestCheckGroupTokenDeletion_ShouldNotDelete(t *testing.T) {
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 3 * 24 * time.Hour}}
	oldToken := groupAccessToken(1, time.Now().Add(-5*24*time.Hour))

	newestTokenMinusTwo := groupAccessToken(2, time.Now().AddDate(0, 0, -2))
	shouldNotDeleteMinusTwo := checkGroupTokenDeletion(repo, oldToken, newestTokenMinusTwo)

	newestTokenToday := groupAccessToken(2, time.Now())
	shouldNotDeleteToday := checkGroupTokenDeletion(repo, oldToken, newestTokenToday)

	newestToken := groupAccessToken(2, time.Now().Add(-4*24*time.Hour))
	shouldNotDeleteNewestToken := checkGroupTokenDeletion(repo, newestToken, newestToken)

	assert.False(t, shouldNotDeleteMinusTwo)
	assert.False(t, shouldNotDeleteToday)
	assert.False(t, shouldNotDeleteNewestToken)
}

func TestCheckGroupTokenDeletion_ShouldNotDeleteTokenWithUnknownCreationDate(t *testing.T) {
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: time.Hour}}
	token := &gitlab.GroupAccessToken{}
	token.ID = 1
	newestToken := groupAccessToken(2, time.Now().Add(-2*time.Hour))

	shouldDelete := checkGroupTokenDeletion(repo, token, newestToken)

	assert.False(t, shouldDelete)
}

func TestDeleteGroupTokens_ShouldNotDeleteWhenOneMatchingTokenExists(t *testing.T) {
	groupName := "platform"
	repo := &repository.Repository{Name: "platform", GroupName: &groupName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/platform":
			_, _ = w.Write([]byte(`{"id":42,"name":"platform"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-platform-only","active":true,"created_at":%q},
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

	err := DeleteGroupTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteGroupTokens_ShouldDeleteOnlyOlderMatchingTokensAfterGracePeriod(t *testing.T) {
	groupName := "platform"
	repo := &repository.Repository{Name: "platform", GroupName: &groupName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/platform":
			_, _ = w.Write([]byte(`{"id":42,"name":"platform"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-platform-oldest","active":true,"created_at":%q},
				{"id":2,"name":"tc-platform-old","active":true,"created_at":%q},
				{"id":3,"name":"tc-platform-newest","active":true,"created_at":%q},
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

	err := DeleteGroupTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"/api/v4/groups/42/access_tokens/1",
		"/api/v4/groups/42/access_tokens/2",
	}, deleteCalls)
}

func TestDeleteGroupTokens_ShouldReturnRevokeErrors(t *testing.T) {
	groupName := "platform"
	repo := &repository.Repository{Name: "platform", GroupName: &groupName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/platform":
			_, _ = w.Write([]byte(`{"id":42,"name":"platform"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-platform-old","active":true,"created_at":%q},
				{"id":2,"name":"tc-platform-newest","active":true,"created_at":%q}
			]`, time.Now().Add(-72*time.Hour).Format(time.RFC3339), time.Now().Add(-48*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			http.Error(w, "revoke failed", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteGroupTokens(client, repo, "tc")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "deleting token")
}

func TestDeleteGroupTokens_ShouldIgnoreRevokedAndInactiveTokens(t *testing.T) {
	groupName := "platform"
	repo := &repository.Repository{Name: "platform", GroupName: &groupName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/platform":
			_, _ = w.Write([]byte(`{"id":42,"name":"platform"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-platform-revoked","active":false,"revoked":true,"created_at":%q},
				{"id":2,"name":"tc-platform-inactive","active":false,"created_at":%q},
				{"id":3,"name":"tc-platform-newest","active":true,"created_at":%q}
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

	err := DeleteGroupTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func TestDeleteGroupTokens_ShouldNotDeleteWhenGracePeriodHasNotPassed(t *testing.T) {
	groupName := "platform"
	repo := &repository.Repository{Name: "platform", GroupName: &groupName, GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	deleteCalls := make([]string, 0)
	client := newGroupTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/platform":
			_, _ = w.Write([]byte(`{"id":42,"name":"platform"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups/42/access_tokens":
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":1,"name":"tc-platform-old","active":true,"created_at":%q},
				{"id":2,"name":"tc-platform-newest","active":true,"created_at":%q}
			]`, time.Now().Add(-20*time.Hour).Format(time.RFC3339), time.Now().Add(-10*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteGroupTokens(client, repo, "tc")

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func groupAccessToken(id int, createdAt time.Time) *gitlab.GroupAccessToken {
	token := &gitlab.GroupAccessToken{}
	token.ID = id
	token.CreatedAt = &createdAt
	token.Active = true
	return token
}
