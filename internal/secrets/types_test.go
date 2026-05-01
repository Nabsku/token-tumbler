package secrets

import (
	"testing"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretStoreForToken(t *testing.T) {
	t.Run("allows explicit none", func(t *testing.T) {
		secret, err := ForRepository(&repository.Repository{SecretStore: "none"})

		require.NoError(t, err)
		assert.Nil(t, secret)
	})

	t.Run("rejects unsupported store", func(t *testing.T) {
		secret, err := ForRepository(&repository.Repository{SecretStore: "file"})

		require.Error(t, err)
		assert.Nil(t, secret)
	})
}
