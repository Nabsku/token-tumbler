package runner

import (
	"context"
	"fmt"
	"testing"

	"github.com/nabsku/token-tumbler/internal/secrets"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

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

type mockSecretStore struct {
	readValue      string
	readErr        error
	writeCalls     []string
	writeErr       error
	writeErrOnCall int // 0 = all calls, N = only on Nth call (1-based)
	writeCallCount int
	writeMetaErr   error
	readMetaValue  secrets.TokenMetadata
	readMetaErr    error
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
