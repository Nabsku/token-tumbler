package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/nabsku/token-tumbler/internal/types/repository"
)

type SecretStore interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context, value string) error
	InitClient(ctx context.Context) error
}

func ForRepository(entry *repository.Repository) (SecretStore, error) {
	switch strings.ToLower(strings.TrimSpace(entry.SecretStore)) {
	case "none":
		return nil, nil
	case "vault":
		if entry.VaultPath == nil || entry.VaultKey == nil || entry.Mount == nil {
			return nil, fmt.Errorf("%w: vaultPath, vaultKey, and vaultMount are required", repository.ErrInvalidRepositoryConfig)
		}
		vaultPath := strings.TrimSpace(*entry.VaultPath)
		vaultKey := strings.TrimSpace(*entry.VaultKey)
		vaultMount := strings.TrimSpace(*entry.Mount)
		if vaultPath == "" || vaultKey == "" || vaultMount == "" {
			return nil, fmt.Errorf("%w: vaultPath, vaultKey, and vaultMount must not be blank", repository.ErrInvalidRepositoryConfig)
		}
		authMethod := ""
		if entry.VaultAuthMethod != nil {
			authMethod = strings.TrimSpace(*entry.VaultAuthMethod)
		}
		authRole := ""
		if entry.VaultAuthRole != nil {
			authRole = strings.TrimSpace(*entry.VaultAuthRole)
		}
		return &VaultSecret{
			Path:       vaultPath,
			Key:        vaultKey,
			MountPath:  vaultMount,
			AuthMethod: authMethod,
			AuthRole:   authRole,
		}, nil
	case "file":
		if entry.FilePath == nil {
			return nil, fmt.Errorf("%w: filePath is required for file secret store", repository.ErrInvalidRepositoryConfig)
		}
		filePath := strings.TrimSpace(*entry.FilePath)
		if filePath == "" {
			return nil, fmt.Errorf("%w: filePath must not be blank", repository.ErrInvalidRepositoryConfig)
		}
		return &FileSecret{Path: filePath}, nil
	case "aws":
		if entry.AWSSecretName == nil || entry.AWSRegion == nil {
			return nil, fmt.Errorf("%w: awsSecretName and awsRegion are required for aws secret store", repository.ErrInvalidRepositoryConfig)
		}
		secretName := strings.TrimSpace(*entry.AWSSecretName)
		region := strings.TrimSpace(*entry.AWSRegion)
		if secretName == "" || region == "" {
			return nil, fmt.Errorf("%w: awsSecretName and awsRegion must not be blank", repository.ErrInvalidRepositoryConfig)
		}
		return &AWSSecret{SecretName: secretName, Region: region}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported secret store %q", repository.ErrInvalidRepositoryConfig, entry.SecretStore)
	}
}
