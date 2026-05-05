package group

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
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

func TestCheckGroupTokenDeletionAt_GracePeriodBoundary(t *testing.T) {
	createdAt := time.Date(2026, time.January, 13, 12, 0, 0, 0, time.UTC)
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	oldToken := groupAccessToken(1, createdAt.Add(-time.Hour))
	preserveToken := groupAccessToken(2, createdAt)

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
			got := checkGroupTokenDeletionAt(repo, oldToken, preserveToken, tt.now)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCheckGroupTokenDeletionAt_ZeroGracePeriodBoundary(t *testing.T) {
	createdAt := time.Date(2026, time.January, 13, 12, 0, 0, 0, time.UTC)
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 0}}
	oldToken := groupAccessToken(1, createdAt.Add(-time.Hour))
	preserveToken := groupAccessToken(2, createdAt)

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
			got := checkGroupTokenDeletionAt(repo, oldToken, preserveToken, tt.now)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolvePreserveToken_GroupPolicy(t *testing.T) {
	older := groupAccessToken(1, time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC))
	newest := groupAccessToken(2, time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC))
	tokens := []*gitlab.GroupAccessToken{older, newest}

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
			got := resolvePreserveToken(tokens, tt.vaultTokenID, zap.NewNop(), "platform")

			require.NotNil(t, got)
			assert.Equal(t, tt.wantID, got.ID)
		})
	}
}

func TestCheckGroupTokenDeletionAt_DeletesNonPreservedTokensAfterGracePeriod(t *testing.T) {
	now := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC)
	repo := &repository.Repository{GracePeriod: &repository.Duration{Duration: 24 * time.Hour}}
	preserveToken := groupAccessToken(1, now.Add(-72*time.Hour))
	newerOrphan := groupAccessToken(2, now.Add(-48*time.Hour))
	olderToken := groupAccessToken(3, now.Add(-96*time.Hour))

	assert.True(t, checkGroupTokenDeletionAt(repo, newerOrphan, preserveToken, now))
	assert.True(t, checkGroupTokenDeletionAt(repo, olderToken, preserveToken, now))
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
				{"id":1,"name":"tt-platform-2026-01-01T00:00:00Z","active":true,"created_at":%q},
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

	err := DeleteGroupTokens(context.Background(), client, repo, "tt", 0)

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
				{"id":1,"name":"tt-platform-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"tt-platform-2026-01-02T00:00:00Z","active":true,"created_at":%q},
				{"id":3,"name":"tt-platform-2026-01-03T00:00:00Z","active":true,"created_at":%q},
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

	err := DeleteGroupTokens(context.Background(), client, repo, "tt", 0)

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
				{"id":1,"name":"tt-platform-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"tt-platform-2026-01-02T00:00:00Z","active":true,"created_at":%q}
			]`, time.Now().Add(-72*time.Hour).Format(time.RFC3339), time.Now().Add(-48*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			http.Error(w, "revoke failed", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteGroupTokens(context.Background(), client, repo, "tt", 0)

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
				{"id":1,"name":"tt-platform-2026-01-01T00:00:00Z","active":false,"revoked":true,"created_at":%q},
				{"id":2,"name":"tt-platform-2026-01-02T00:00:00Z","active":false,"created_at":%q},
				{"id":3,"name":"tt-platform-2026-01-03T00:00:00Z","active":true,"created_at":%q}
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

	err := DeleteGroupTokens(context.Background(), client, repo, "tt", 0)

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
				{"id":1,"name":"tt-platform-2026-01-01T00:00:00Z","active":true,"created_at":%q},
				{"id":2,"name":"tt-platform-2026-01-02T00:00:00Z","active":true,"created_at":%q}
			]`, time.Now().Add(-20*time.Hour).Format(time.RFC3339), time.Now().Add(-10*time.Hour).Format(time.RFC3339))))
		case r.Method == http.MethodDelete:
			deleteCalls = append(deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	})

	err := DeleteGroupTokens(context.Background(), client, repo, "tt", 0)

	require.NoError(t, err)
	assert.Empty(t, deleteCalls)
}

func groupAccessToken(id int64, createdAt time.Time) *gitlab.GroupAccessToken {
	token := &gitlab.GroupAccessToken{}
	token.ID = id
	token.CreatedAt = &createdAt
	token.Active = true
	return token
}
