package secrets

import (
	"context"
	vault "github.com/hashicorp/vault/api"
)

type (
	SecretWrapper struct {
		*vault.KVSecret
	}
	ClientWrapper struct {
		*vault.Client
	}
	SecretStore interface {
		Read(ctx context.Context) (string, error)
		Write(ctx context.Context) error
		InitClient(ctx context.Context) error
	}
)
