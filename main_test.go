package main

import (
	"os"
	"testing"

	"github.com/nabsku/token-chaser/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/api/client-go"
)

func TestCheckEnvVars(t *testing.T) {
	t.Run("returns nil when all variables exist", func(t *testing.T) {
		t.Setenv("TOKEN_CHASER_MAIN_TEST_A", "a")
		t.Setenv("TOKEN_CHASER_MAIN_TEST_B", "b")

		err := checkEnvVars("TOKEN_CHASER_MAIN_TEST_A", "TOKEN_CHASER_MAIN_TEST_B")

		require.NoError(t, err)
	})

	t.Run("returns joined missing variables", func(t *testing.T) {
		err := checkEnvVars("TOKEN_CHASER_MAIN_TEST_MISSING_A", "TOKEN_CHASER_MAIN_TEST_MISSING_B")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_CHASER_MAIN_TEST_MISSING_A, TOKEN_CHASER_MAIN_TEST_MISSING_B")
	})

	t.Run("treats empty variables as missing", func(t *testing.T) {
		t.Setenv("TOKEN_CHASER_MAIN_TEST_EMPTY", "")

		err := checkEnvVars("TOKEN_CHASER_MAIN_TEST_EMPTY")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_CHASER_MAIN_TEST_EMPTY")
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
	require.NoError(t, os.WriteFile("config.yaml", []byte(`prefix: tc
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

func TestMatchingProjectTokens_ShouldSkipForeignTokens(t *testing.T) {
	repo := &repository.Repository{RepoName: gitlab.Ptr("service"), Name: "service"}
	tokens := []*gitlab.ProjectAccessToken{
		projectTokenNamed("foreign-token"),
		revokedProjectTokenNamed("tc-service-revoked"),
		inactiveProjectTokenNamed("tc-service-inactive"),
		projectTokenNamed("tc-service-2026-01-01T00:00:00Z"),
	}

	got := matchingProjectTokens(tokens, repo, "tc", 0)

	require.Len(t, got, 1)
	assert.Equal(t, "tc-service-2026-01-01T00:00:00Z", got[0].Name)
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

func TestSecretStoreForToken(t *testing.T) {
	t.Run("allows explicit none", func(t *testing.T) {
		secret, err := secretStoreForToken(&repository.Repository{SecretStore: "none"}, "token")

		require.NoError(t, err)
		assert.Nil(t, secret)
	})

	t.Run("rejects unsupported store", func(t *testing.T) {
		secret, err := secretStoreForToken(&repository.Repository{SecretStore: "file"}, "token")

		require.Error(t, err)
		assert.Nil(t, secret)
	})
}
