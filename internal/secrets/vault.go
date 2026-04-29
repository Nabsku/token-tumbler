package secrets

import (
	"context"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/approle"
	"os"
)

type VaultSecret struct {
	Path      string
	Key       string
	Value     string
	MountPath string
	Client    *vault.Client
}

func (vs *VaultSecret) InitClient(ctx context.Context) error {
	config := vault.DefaultConfig()

	client, err := vault.NewClient(config)
	if err != nil {
		return fmt.Errorf("unable to initialize a new vault client: %w", err)
	}

	appRoleID := os.Getenv("APPROLE_ID")
	appRoleSecret := &auth.SecretID{
		FromEnv: "APPROLE_SECRET",
	}

	appRoleAuth, err := auth.NewAppRoleAuth(appRoleID, appRoleSecret)
	if err != nil {
		return fmt.Errorf("unable to initialize AppRole auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return fmt.Errorf("unable to login to AppRole auth method: %w", err)
	}
	if authInfo == nil {
		return fmt.Errorf("no auth info was returned after login")
	}

	client.SetToken(authInfo.Auth.ClientToken)
	vs.Client = client

	return nil
}

func (vs *VaultSecret) Read(ctx context.Context) (string, error) {
	err := vs.InitClient(ctx)
	if err != nil {
		return "", err
	}

	secret, err := vs.Client.KVv2(vs.MountPath).Get(ctx, vs.Path)
	if err != nil {
		return "", err
	}

	s, ok := secret.Data[vs.Key]
	if !ok {
		return "", fmt.Errorf("secret does not contain key %s", vs.Key)
	}

	if _, ok := s.(string); !ok {
		return "", fmt.Errorf("secret %s is not a string, not overwriting", vs.Key)
	}

	return s.(string), nil
}

func (vs *VaultSecret) Write(ctx context.Context) error {
	err := vs.InitClient(ctx)
	if err != nil {
		return err
	}
	secretData := map[string]interface{}{
		vs.Key: vs.Value,
	}
	_, errPut := vs.Client.KVv2(vs.MountPath).Put(ctx, vs.Path, secretData)
	if errPut != nil {
		return errPut
	}
	return nil
}
