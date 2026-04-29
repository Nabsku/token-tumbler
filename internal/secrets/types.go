package secrets

import "context"

type SecretStore interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context) error
	InitClient(ctx context.Context) error
}
