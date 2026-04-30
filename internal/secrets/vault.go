package secrets

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	vault "github.com/hashicorp/vault/api"
	approle "github.com/hashicorp/vault/api/auth/approle"
	awsauth "github.com/hashicorp/vault/api/auth/aws"
	k8sauth "github.com/hashicorp/vault/api/auth/kubernetes"
)

type VaultSecret struct {
	Path       string
	Key        string
	MountPath  string
	AuthMethod string
	AuthRole   string
	Client     *vault.Client
}

func (vs *VaultSecret) InitClient(ctx context.Context) error {
	config := vault.DefaultConfig()

	client, err := vault.NewClient(config)
	if err != nil {
		return fmt.Errorf("unable to initialize a new vault client: %w", err)
	}

	authMethod := strings.ToLower(strings.TrimSpace(vs.AuthMethod))
	if authMethod == "" {
		authMethod = "approle"
	}

	var token string
	switch authMethod {
	case "approle":
		token, err = vs.loginAppRole(ctx, client)
	case "token":
		token, err = vs.loginToken()
	case "kubernetes":
		token, err = vs.loginKubernetes(ctx, client)
	case "aws":
		token, err = vs.loginAWS(ctx, client)
	default:
		return fmt.Errorf("unsupported vault auth method %q", authMethod)
	}
	if err != nil {
		return err
	}

	client.SetToken(token)
	vs.Client = client
	return nil
}

func (vs *VaultSecret) loginAppRole(ctx context.Context, client *vault.Client) (string, error) {
	appRoleID := strings.TrimSpace(os.Getenv("APPROLE_ID"))
	if appRoleID == "" {
		return "", fmt.Errorf("unable to initialize AppRole auth method: APPROLE_ID is required")
	}
	if strings.TrimSpace(os.Getenv("APPROLE_SECRET")) == "" {
		return "", fmt.Errorf("unable to initialize AppRole auth method: APPROLE_SECRET is required")
	}

	appRoleSecret := &approle.SecretID{
		FromEnv: "APPROLE_SECRET",
	}

	appRoleAuth, err := approle.NewAppRoleAuth(appRoleID, appRoleSecret)
	if err != nil {
		return "", fmt.Errorf("unable to initialize AppRole auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(ctx, appRoleAuth)
	if err != nil {
		return "", fmt.Errorf("unable to login to AppRole auth method: %w", err)
	}
	if authInfo == nil || authInfo.Auth == nil || authInfo.Auth.ClientToken == "" {
		return "", fmt.Errorf("no auth info was returned after login")
	}
	return authInfo.Auth.ClientToken, nil
}

func (vs *VaultSecret) loginToken() (string, error) {
	token := strings.TrimSpace(os.Getenv("VAULT_TOKEN"))
	if token == "" {
		return "", fmt.Errorf("VAULT_TOKEN is required for token auth method")
	}
	return token, nil
}

func (vs *VaultSecret) loginKubernetes(ctx context.Context, client *vault.Client) (string, error) {
	role := strings.TrimSpace(vs.AuthRole)
	if role == "" {
		return "", fmt.Errorf("vaultAuthRole is required for kubernetes auth method")
	}

	opts := []k8sauth.LoginOption{}
	if tokenPath := os.Getenv("VAULT_K8S_TOKEN_PATH"); tokenPath != "" {
		opts = append(opts, k8sauth.WithServiceAccountTokenPath(tokenPath))
	}

	k8sAuth, err := k8sauth.NewKubernetesAuth(role, opts...)
	if err != nil {
		return "", fmt.Errorf("unable to initialize Kubernetes auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(ctx, k8sAuth)
	if err != nil {
		return "", fmt.Errorf("unable to login to Kubernetes auth method: %w", err)
	}
	if authInfo == nil || authInfo.Auth == nil || authInfo.Auth.ClientToken == "" {
		return "", fmt.Errorf("no auth info was returned after login")
	}
	return authInfo.Auth.ClientToken, nil
}

func (vs *VaultSecret) loginAWS(ctx context.Context, client *vault.Client) (string, error) {
	role := strings.TrimSpace(vs.AuthRole)
	if role == "" {
		return "", fmt.Errorf("vaultAuthRole is required for aws auth method")
	}

	awsAuth, err := awsauth.NewAWSAuth(awsauth.WithRole(role), awsauth.WithIAMAuth())
	if err != nil {
		return "", fmt.Errorf("unable to initialize AWS auth method: %w", err)
	}

	authInfo, err := client.Auth().Login(ctx, awsAuth)
	if err != nil {
		return "", fmt.Errorf("unable to login to AWS auth method: %w", err)
	}
	if authInfo == nil || authInfo.Auth == nil || authInfo.Auth.ClientToken == "" {
		return "", fmt.Errorf("no auth info was returned after login")
	}
	return authInfo.Auth.ClientToken, nil
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
