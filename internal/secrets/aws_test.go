package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ SecretStore = (*AWSSecret)(nil)

func TestForRepository_ShouldRejectBlankAWSConfig(t *testing.T) {
	secretName := "  "
	region := "us-east-1"
	entry := &repository.Repository{
		SecretStore:   "aws",
		AWSSecretName: &secretName,
		AWSRegion:     &region,
	}

	store, err := ForRepository(entry)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repository.ErrInvalidRepositoryConfig))
	assert.Contains(t, err.Error(), "must not be blank")
	assert.Nil(t, store)
}

func TestForRepository_ShouldTrimAWSConfig(t *testing.T) {
	secretName := "  my-secret  "
	region := "  us-west-2  "
	entry := &repository.Repository{
		SecretStore:   "aws",
		AWSSecretName: &secretName,
		AWSRegion:     &region,
	}

	store, err := ForRepository(entry)

	require.NoError(t, err)
	secret, ok := store.(*AWSSecret)
	require.True(t, ok)
	assert.Equal(t, "my-secret", secret.SecretName)
	assert.Equal(t, "us-west-2", secret.Region)
}

func TestAWSSecret_InitClient_ShouldReturnErrorForBlankRegion(t *testing.T) {
	secret := &AWSSecret{SecretName: "my-secret", Region: "  "}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "awsRegion must not be blank")
}

func TestAWSSecret_Read_ShouldReturnErrorForBlankSecretName(t *testing.T) {
	secret := &AWSSecret{SecretName: "  ", Region: "us-east-1"}

	_, err := secret.Read(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "awsSecretName must not be blank")
}

func TestAWSSecret_Write_ShouldReturnErrorForBlankSecretName(t *testing.T) {
	secret := &AWSSecret{SecretName: "  ", Region: "us-east-1"}

	err := secret.Write(context.Background(), "token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "awsSecretName must not be blank")
}
