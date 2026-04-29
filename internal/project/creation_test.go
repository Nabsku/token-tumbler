package project

import (
	"testing"
	"time"

	"github.com/nabsku/token-chaser/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateProjectAccessTokenOptions(t *testing.T) {
	expiry := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	scopes := []string{"api", "read_repository"}

	opts := createProjectAccessTokenOptions("tc-service-2026", scopes, &expiry)

	require.NotNil(t, opts)
	require.NotNil(t, opts.Name)
	require.NotNil(t, opts.Scopes)
	require.NotNil(t, opts.ExpiresAt)
	assert.Equal(t, "tc-service-2026", *opts.Name)
	assert.Equal(t, scopes, *opts.Scopes)
	assert.True(t, time.Time(*opts.ExpiresAt).Equal(expiry))
}

func TestRenewProjectAccessToken_ShouldValidateTokenNameBeforeGitLabCall(t *testing.T) {
	token, err := RenewProjectAccessToken(nil, 1, &repository.Repository{}, "tc")

	require.Error(t, err)
	assert.Nil(t, token)
	assert.Contains(t, err.Error(), "cannot be empty")
}
