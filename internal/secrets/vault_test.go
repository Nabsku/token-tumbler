package secrets

import (
	"context"
	"errors"
	"testing"

	vault "github.com/hashicorp/vault/api"
	"github.com/nabsku/token-tumbler/internal/types/repository"
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

func TestForRepository_ShouldRejectBlankVaultConfig(t *testing.T) {
	vaultPath := "  "
	vaultKey := "gitlab_token"
	vaultMount := "kv"
	entry := &repository.Repository{
		SecretStore: "vault",
		VaultPath:   &vaultPath,
		VaultKey:    &vaultKey,
		Mount:       &vaultMount,
	}

	store, err := ForRepository(entry)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repository.ErrInvalidRepositoryConfig))
	assert.Contains(t, err.Error(), "must not be blank")
	assert.Nil(t, store)
}

func TestForRepository_ShouldTrimVaultConfig(t *testing.T) {
	vaultPath := "  gitlab/project  "
	vaultKey := "  gitlab_token  "
	vaultMount := "  kv  "
	entry := &repository.Repository{
		SecretStore: "vault",
		VaultPath:   &vaultPath,
		VaultKey:    &vaultKey,
		Mount:       &vaultMount,
	}

	store, err := ForRepository(entry)

	require.NoError(t, err)
	secret, ok := store.(*VaultSecret)
	require.True(t, ok)
	assert.Equal(t, "gitlab/project", secret.Path)
	assert.Equal(t, "gitlab_token", secret.Key)
	assert.Equal(t, "kv", secret.MountPath)
}

func TestVaultSecret_Write_ShouldWrapInitClientErrors(t *testing.T) {
	t.Setenv("APPROLE_ID", "role-id")
	t.Setenv("APPROLE_SECRET", "")
	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv"}

	err := secret.Write(context.Background(), "token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "initializing vault client")
	assert.Contains(t, err.Error(), "APPROLE_SECRET is required")
	assert.Nil(t, secret.Client)
}

func TestMergeSecretData_ShouldPreserveUnrelatedKeys(t *testing.T) {
	existing := &vault.KVSecret{Data: map[string]interface{}{
		"gitlab_token": "old-token",
		"username":     "deploy-bot",
		"retries":      float64(3),
	}}

	got := mergeSecretData(existing, "gitlab_token", "new-token")

	assert.Equal(t, map[string]interface{}{
		"gitlab_token": "new-token",
		"username":     "deploy-bot",
		"retries":      float64(3),
	}, got)
	assert.Equal(t, "old-token", existing.Data["gitlab_token"])
}

func TestMergeSecretData_ShouldCreateSecretDataWhenNoExistingSecret(t *testing.T) {
	got := mergeSecretData(nil, "gitlab_token", "new-token")

	assert.Equal(t, map[string]interface{}{"gitlab_token": "new-token"}, got)
}

func TestIsVaultNotFound_ShouldRecognizeKVSecretNotFound(t *testing.T) {
	assert.True(t, isVaultNotFound(vault.ErrSecretNotFound))
}

func TestVaultSecret_InitClient_ShouldReturnErrorForMissingToken(t *testing.T) {
	t.Setenv("VAULT_TOKEN", "")
	secret := &VaultSecret{AuthMethod: "token"}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "VAULT_TOKEN is required")
	assert.Nil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldUseToken(t *testing.T) {
	t.Setenv("VAULT_TOKEN", "my-test-token")
	secret := &VaultSecret{AuthMethod: "token"}

	err := secret.InitClient(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldReturnErrorForMissingK8sRole(t *testing.T) {
	secret := &VaultSecret{AuthMethod: "kubernetes"}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vaultAuthRole is required")
	assert.Nil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldReturnErrorForMissingAwsRole(t *testing.T) {
	secret := &VaultSecret{AuthMethod: "aws"}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vaultAuthRole is required")
	assert.Nil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldDefaultToAppRole(t *testing.T) {
	t.Setenv("APPROLE_ID", "")
	secret := &VaultSecret{}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "APPROLE_ID is required")
}
