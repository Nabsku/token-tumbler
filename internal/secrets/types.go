package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/nabsku/token-tumbler/internal/types/repository"
)

type SecretStore interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context) error
	InitClient(ctx context.Context) error
}

func ForRepository(entry *repository.Repository, token string) (SecretStore, error) {
	switch strings.ToLower(strings.TrimSpace(entry.SecretStore)) {
	case "none":
		return nil, nil
	case "vault":
		if entry.VaultPath == nil || entry.VaultKey == nil || entry.Mount == nil {
			return nil, fmt.Errorf("%w: vaultPath, vaultKey, and vaultMount are required", repository.ErrInvalidRepositoryConfig)
		}
		return &VaultSecret{
			Path:      *entry.VaultPath,
			Key:       *entry.VaultKey,
			Value:     token,
			MountPath: *entry.Mount,
		}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported secret store %q", repository.ErrInvalidRepositoryConfig, entry.SecretStore)
	}
}
