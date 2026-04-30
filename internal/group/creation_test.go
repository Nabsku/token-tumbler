package group

import (
	"errors"
	"testing"
	"time"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestCreateGroupAccessTokenOptions(t *testing.T) {
	expiry := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	scopes := []string{"api", "read_registry"}

	opts := createGroupAccessTokenOptions("tt-group-2026", scopes, &expiry)

	require.NotNil(t, opts)
	require.NotNil(t, opts.Name)
	require.NotNil(t, opts.Scopes)
	require.NotNil(t, opts.ExpiresAt)
	assert.Equal(t, "tt-group-2026", *opts.Name)
	assert.Equal(t, scopes, *opts.Scopes)
	assert.True(t, time.Time(*opts.ExpiresAt).Equal(expiry))
}

func TestRenewGroupAccessToken_ShouldValidateTokenNameBeforeGitLabCall(t *testing.T) {
	token, err := RenewGroupAccessToken(nil, 1, &repository.Repository{}, "tt")

	require.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "cannot be empty")
}

func TestValidateGroupAccessTokenResponse(t *testing.T) {
	tests := []struct {
		name  string
		token *gitlab.GroupAccessToken
	}{
		{name: "nil token"},
		{name: "missing ID", token: groupAccessTokenResponse(0, "secret")},
		{name: "missing token value", token: groupAccessTokenResponse(1, "")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGroupAccessTokenResponse(tt.token)

			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrInvalidGroupTokenResponse))
		})
	}

	err := validateGroupAccessTokenResponse(groupAccessTokenResponse(1, "secret"))
	require.NoError(t, err)
}

func groupAccessTokenResponse(id int64, tokenValue string) *gitlab.GroupAccessToken {
	token := &gitlab.GroupAccessToken{}
	token.ID = id
	token.Token = tokenValue
	return token
}
