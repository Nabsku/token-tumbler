package config

import (
	"os"
	"testing"
	"time"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("config.yaml", []byte(`prefix: tt
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

	cfg, err := ReadRepositoryConfig("config.yaml")

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.Prefix)
	assert.NotEmpty(t, cfg.Repos)
	for _, repo := range cfg.Repos {
		assert.NotEmpty(t, repo.Name)
		assert.NotZero(t, repo.Lifetime.ToDuration())
		if repo.RotationThreshold != nil {
			assert.NoError(t, (&repository.Repository{Lifetime: repo.Lifetime, RotationThreshold: repo.RotationThreshold}).CheckKeyRotationAndTokenAge())
		}
	}
}

func TestCheckedInConfig_ShouldValidate(t *testing.T) {
	buff, err := os.ReadFile("../../config.example.yaml")
	require.NoError(t, err)

	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("config.yaml", buff, 0o600))

	cfg, err := ReadRepositoryConfig("config.yaml")

	require.NoError(t, err)
	require.NotNil(t, cfg)
}

func TestPollIntervalFromEnv(t *testing.T) {
	t.Run("defaults to production interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "")

		interval, err := PollIntervalFromEnv()

		require.NoError(t, err)
		assert.Equal(t, defaultPollInterval, interval)
	})

	t.Run("uses environment interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "15m")

		interval, err := PollIntervalFromEnv()

		require.NoError(t, err)
		assert.Equal(t, 15*time.Minute, interval)
	})

	t.Run("rejects invalid interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "soon")

		interval, err := PollIntervalFromEnv()

		require.Error(t, err)
		assert.Zero(t, interval)
	})

	t.Run("rejects non-positive interval", func(t *testing.T) {
		t.Setenv(pollIntervalEnvVar, "0s")

		interval, err := PollIntervalFromEnv()

		require.Error(t, err)
		assert.Zero(t, interval)
	})
}
