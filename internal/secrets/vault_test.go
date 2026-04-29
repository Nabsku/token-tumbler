package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ SecretStore = (*VaultSecret)(nil)

func TestVaultSecret_InitClient_ShouldReturnErrorForInvalidAppRoleConfig(t *testing.T) {
	t.Setenv("APPROLE_ID", "")
	t.Setenv("APPROLE_SECRET", "secret")

	secret := &VaultSecret{}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to initialize AppRole auth method")
	assert.Nil(t, secret.Client)
}
