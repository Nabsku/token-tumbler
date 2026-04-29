package main

import (
	"os"
	"testing"

	"github.com/nabsku/token-chaser/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckEnvVars(t *testing.T) {
	t.Run("returns nil when all variables exist", func(t *testing.T) {
		t.Setenv("TOKEN_CHASER_MAIN_TEST_A", "a")
		t.Setenv("TOKEN_CHASER_MAIN_TEST_B", "")

		err := checkEnvVars("TOKEN_CHASER_MAIN_TEST_A", "TOKEN_CHASER_MAIN_TEST_B")

		require.NoError(t, err)
	})

	t.Run("returns joined missing variables", func(t *testing.T) {
		err := checkEnvVars("TOKEN_CHASER_MAIN_TEST_MISSING_A", "TOKEN_CHASER_MAIN_TEST_MISSING_B")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TOKEN_CHASER_MAIN_TEST_MISSING_A, TOKEN_CHASER_MAIN_TEST_MISSING_B")
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
