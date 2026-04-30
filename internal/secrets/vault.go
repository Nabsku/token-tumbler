package secrets

import (
	"context"
	"errors"
	"fmt"
	vault "github.com/hashicorp/vault/api"
	auth "github.com/hashicorp/vault/api/auth/approle"
	"net/http"
	"os"
	"strings"
)

type VaultSecret struct {
	Path      string
	Key       string
	MountPath string
	Client    *vault.Client
}

func (vs *VaultSecret) InitClient(ctx context.Context) error {
	config := vault.DefaultConfig()

	client, err := vault.NewClient(config)
	if err != nil {
		return fmt.Errorf("unable to initialize a new vault client: %w", err)
	}

	appRoleID := strings.TrimSpace(os.Getenv("APPROLE_ID"))
	if appRoleID == "" {
		return fmt.Errorf("unable to initialize AppRole auth method: APPROLE_ID is required")
	}
	if strings.TrimSpace(os.Getenv("APPROLE_SECRET")) == "" {
		return fmt.Errorf("unable to initialize AppRole auth method: APPROLE_SECRET is required")
	}
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
	if authInfo == nil || authInfo.Auth == nil || authInfo.Auth.ClientToken == "" {
		return fmt.Errorf("no auth info was returned after login")
	}

	client.SetToken(authInfo.Auth.ClientToken)
	vs.Client = client

	return nil
}

func (vs *VaultSecret) Read(ctx context.Context) (string, error) {
	err := vs.InitClient(ctx)
	if err != nil {
		return "", fmt.Errorf("initializing vault client: %w", err)
	}

	secret, err := vs.Client.KVv2(vs.MountPath).Get(ctx, vs.Path)
	if err != nil {
		return "", fmt.Errorf("reading vault secret %s/%s: %w", vs.MountPath, vs.Path, err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("secret %s does not exist", vs.Path)
	}

	secretValue, ok := secret.Data[vs.Key]
	if !ok {
		return "", fmt.Errorf("secret does not contain key %s", vs.Key)
	}
	value, ok := secretValue.(string)
	if !ok {
		return "", fmt.Errorf("secret %s is not a string, not overwriting", vs.Key)
	}
	return value, nil
}

func (vs *VaultSecret) Write(ctx context.Context, value string) error {
	err := vs.InitClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing vault client: %w", err)
	}
	secretData, err := vs.mergedSecretData(ctx, value)
	if err != nil {
		return fmt.Errorf("preparing vault secret %s/%s: %w", vs.MountPath, vs.Path, err)
	}
	_, errPut := vs.Client.KVv2(vs.MountPath).Put(ctx, vs.Path, secretData)
	if errPut != nil {
		return fmt.Errorf("writing vault secret %s/%s key %s: %w", vs.MountPath, vs.Path, vs.Key, errPut)
	}
	return nil
}

func (vs *VaultSecret) mergedSecretData(ctx context.Context, value string) (map[string]interface{}, error) {
	secret, err := vs.Client.KVv2(vs.MountPath).Get(ctx, vs.Path)
	if err != nil && !isVaultNotFound(err) {
		return nil, fmt.Errorf("reading existing vault secret %s/%s before merge: %w", vs.MountPath, vs.Path, err)
	}

	return mergeSecretData(secret, vs.Key, value), nil
}

func isVaultNotFound(err error) bool {
	if errors.Is(err, vault.ErrSecretNotFound) {
		return true
	}
	var responseError *vault.ResponseError
	return errors.As(err, &responseError) && responseError.StatusCode == http.StatusNotFound
}

func mergeSecretData(secret *vault.KVSecret, key string, value string) map[string]interface{} {
	secretData := make(map[string]interface{})
	if secret != nil {
		for existingKey, existingValue := range secret.Data {
			secretData[existingKey] = existingValue
		}
	}
	secretData[key] = value

	return secretData
}
