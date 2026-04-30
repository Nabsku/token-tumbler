package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nabsku/token-tumbler/internal/helper"
	"github.com/nabsku/token-tumbler/internal/types/repository"
)

type TokenMetadata struct {
	TokenID   int64     `json:"token_id"`
	TokenName string    `json:"token_name"`
	CreatedAt time.Time `json:"created_at"`
}

func parseTokenMetadata(data string) (TokenMetadata, error) {
	var meta TokenMetadata
	if err := json.Unmarshal([]byte(data), &meta); err != nil {
		return TokenMetadata{}, err
	}
	return meta, nil
}

func formatTokenMetadata(meta TokenMetadata) (string, error) {
	b, err := json.Marshal(meta)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type SecretStore interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context, value string) error
	InitClient(ctx context.Context) error
	ReadMetadata(ctx context.Context) (TokenMetadata, error)
	WriteMetadata(ctx context.Context, meta TokenMetadata) error
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
		if err := helper.ValidateSecureFilePath(filePath); err != nil {
			return nil, fmt.Errorf("%w: invalid file secret path: %v", repository.ErrInvalidRepositoryConfig, err)
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
	case "k8s":
		if entry.K8sNamespace == nil || entry.K8sSecretName == nil || entry.K8sSecretKey == nil {
			return nil, fmt.Errorf("%w: k8sNamespace, k8sSecretName, and k8sSecretKey are required for k8s secret store", repository.ErrInvalidRepositoryConfig)
		}
		namespace := strings.TrimSpace(*entry.K8sNamespace)
		secretName := strings.TrimSpace(*entry.K8sSecretName)
		secretKey := strings.TrimSpace(*entry.K8sSecretKey)
		if namespace == "" || secretName == "" || secretKey == "" {
			return nil, fmt.Errorf("%w: k8sNamespace, k8sSecretName, and k8sSecretKey must not be blank", repository.ErrInvalidRepositoryConfig)
		}
		return &K8sSecret{Namespace: namespace, SecretName: secretName, SecretKey: secretKey}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported secret store %q", repository.ErrInvalidRepositoryConfig, entry.SecretStore)
	}
}
