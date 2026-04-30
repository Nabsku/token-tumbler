package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ SecretStore = (*FileSecret)(nil)

func TestForRepository_ShouldRejectBlankFilePath(t *testing.T) {
	filePath := "  "
	entry := &repository.Repository{SecretStore: "file", FilePath: &filePath}

	store, err := ForRepository(entry)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repository.ErrInvalidRepositoryConfig))
	assert.Contains(t, err.Error(), "filePath must not be blank")
	assert.Nil(t, store)
}

func TestForRepository_ShouldTrimFilePath(t *testing.T) {
	filePath := "  /run/secrets/gitlab-token  "
	entry := &repository.Repository{SecretStore: "file", FilePath: &filePath}

	store, err := ForRepository(entry)

	require.NoError(t, err)
	secret, ok := store.(*FileSecret)
	require.True(t, ok)
	assert.Equal(t, "/run/secrets/gitlab-token", secret.Path)
}

func TestFileSecret_Write_ShouldWriteTokenWithRestrictedPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitlab-token")
	secret := &FileSecret{Path: path}

	err := secret.Write(context.Background(), "new-token")

	require.NoError(t, err)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new-token", string(contents))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(fileSecretMode), info.Mode().Perm())
}

func TestFileSecret_Write_ShouldOverwriteExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitlab-token")
	require.NoError(t, os.WriteFile(path, []byte("old-token"), 0o644))
	secret := &FileSecret{Path: path}

	err := secret.Write(context.Background(), "new-token")

	require.NoError(t, err)
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new-token", string(contents))
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(fileSecretMode), info.Mode().Perm())
}

func TestFileSecret_Write_ShouldFailWhenParentDirectoryDoesNotExist(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	path := filepath.Join(dir, "gitlab-token")
	secret := &FileSecret{Path: path}

	err := secret.Write(context.Background(), "new-token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking file secret parent directory")
	assert.NoFileExists(t, path)
}

func TestFileSecret_Read_ShouldReturnFileContents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gitlab-token")
	require.NoError(t, os.WriteFile(path, []byte("stored-token"), fileSecretMode))
	secret := &FileSecret{Path: path}

	got, err := secret.Read(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "stored-token", got)
}
