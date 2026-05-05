package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	vault "github.com/hashicorp/vault/api"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ SecretStore = (*VaultSecret)(nil)

func TestVaultSecret_InitClient_ShouldReturnErrorForInvalidAppRoleConfig(t *testing.T) {
	t.Setenv("APPROLE_ID", "")
	t.Setenv("APPROLE_SECRET", "secret")

	secret := &VaultSecret{}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to initialize AppRole auth method")
	assert.Nil(t, secret.Client)
}

func TestForRepository_ShouldRejectBlankVaultConfig(t *testing.T) {
	vaultPath := "  "
	vaultKey := "gitlab_token"
	vaultMount := "kv"
	entry := &repository.Repository{
		SecretStore: "vault",
		VaultPath:   &vaultPath,
		VaultKey:    &vaultKey,
		Mount:       &vaultMount,
	}

	store, err := ForRepository(entry)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repository.ErrInvalidRepositoryConfig))
	assert.Contains(t, err.Error(), "must not be blank")
	assert.Nil(t, store)
}

func TestForRepository_ShouldTrimVaultConfig(t *testing.T) {
	vaultPath := "  gitlab/project  "
	vaultKey := "  gitlab_token  "
	vaultMount := "  kv  "
	entry := &repository.Repository{
		SecretStore: "vault",
		VaultPath:   &vaultPath,
		VaultKey:    &vaultKey,
		Mount:       &vaultMount,
	}

	store, err := ForRepository(entry)

	require.NoError(t, err)
	secret, ok := store.(*VaultSecret)
	require.True(t, ok)
	assert.Equal(t, "gitlab/project", secret.Path)
	assert.Equal(t, "gitlab_token", secret.Key)
	assert.Equal(t, "kv", secret.MountPath)
}

func TestVaultSecret_Write_ShouldWrapInitClientErrors(t *testing.T) {
	t.Setenv("APPROLE_ID", "role-id")
	t.Setenv("APPROLE_SECRET", "")
	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv"}

	err := secret.Write(context.Background(), "token")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "initializing vault client")
	assert.Contains(t, err.Error(), "APPROLE_SECRET is required")
	assert.Nil(t, secret.Client)
}

func TestMergeSecretData_ShouldPreserveUnrelatedKeys(t *testing.T) {
	existing := &vault.KVSecret{Data: map[string]interface{}{
		"gitlab_token": "old-token",
		"username":     "deploy-bot",
		"retries":      float64(3),
	}}

	got := mergeSecretData(existing, "gitlab_token", "new-token")

	assert.Equal(t, map[string]interface{}{
		"gitlab_token": "new-token",
		"username":     "deploy-bot",
		"retries":      float64(3),
	}, got)
	assert.Equal(t, "old-token", existing.Data["gitlab_token"])
}

func TestMergeSecretData_ShouldCreateSecretDataWhenNoExistingSecret(t *testing.T) {
	got := mergeSecretData(nil, "gitlab_token", "new-token")

	assert.Equal(t, map[string]interface{}{"gitlab_token": "new-token"}, got)
}

func TestIsVaultNotFound_ShouldRecognizeKVSecretNotFound(t *testing.T) {
	assert.True(t, isVaultNotFound(vault.ErrSecretNotFound))
}

func TestVaultSecret_InitClient_ShouldReturnErrorForMissingToken(t *testing.T) {
	t.Setenv("VAULT_TOKEN", "")
	secret := &VaultSecret{AuthMethod: "token"}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "VAULT_TOKEN is required")
	assert.Nil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldUseToken(t *testing.T) {
	t.Setenv("VAULT_TOKEN", "my-test-token")
	secret := &VaultSecret{AuthMethod: "token"}

	err := secret.InitClient(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldReturnErrorForMissingK8sRole(t *testing.T) {
	secret := &VaultSecret{AuthMethod: "kubernetes"}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vaultAuthRole is required")
	assert.Nil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldReturnErrorForMissingAwsRole(t *testing.T) {
	secret := &VaultSecret{AuthMethod: "aws"}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "vaultAuthRole is required")
	assert.Nil(t, secret.Client)
}

func TestVaultSecret_InitClient_ShouldDefaultToAppRole(t *testing.T) {
	t.Setenv("APPROLE_ID", "")
	secret := &VaultSecret{}

	err := secret.InitClient(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "APPROLE_ID is required")
}

func TestIsCASConflict_ShouldRecognizeBadRequest(t *testing.T) {
	assert.True(t, isCASConflict(&vault.ResponseError{StatusCode: 400}))
	assert.False(t, isCASConflict(&vault.ResponseError{StatusCode: 404}))
	assert.False(t, isCASConflict(&vault.ResponseError{StatusCode: 500}))
	assert.False(t, isCASConflict(errors.New("some error")))
}

func TestVaultSecret_Write_ShouldRetryOnCASConflict(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"old-token"},"metadata":{"version":1}}}`))
		case http.MethodPut:
			callCount++
			body, _ := io.ReadAll(r.Body)
			if callCount == 1 {
				assert.Contains(t, string(body), `"options":{"cas":1}`)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errors":["check-and-set parameter did not match the current version"]}`))
				return
			}
			assert.Contains(t, string(body), `"options":{"cas":1}`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"metadata":{"version":2}}}`))
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestVaultSecret_Write_ShouldReturnErrorOnNonCASFailure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"old-token"},"metadata":{"version":1}}}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":["internal server error"]}`))
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "writing vault secret")
}

func TestVaultSecret_Write_ShouldReturnErrorAfterMaxCASRetries(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"old-token"},"metadata":{"version":1}}}`))
			return
		}
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":["check-and-set parameter did not match the current version"]}`))
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CAS conflict after 3 retries")
	assert.Equal(t, 3, callCount)
}

func TestVaultSecret_WriteMetadata_ShouldRetryOnCASConflict(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"old-token","token_id":1},"metadata":{"version":1}}}`))
		case http.MethodPut:
			callCount++
			body, _ := io.ReadAll(r.Body)
			if callCount == 1 {
				assert.Contains(t, string(body), `"options":{"cas":1}`)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"errors":["check-and-set parameter did not match the current version"]}`))
				return
			}
			assert.Contains(t, string(body), `"options":{"cas":1}`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"metadata":{"version":2}}}`))
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.WriteMetadata(context.Background(), TokenMetadata{TokenID: 2, TokenName: "new-token"})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestVaultSecret_DeleteCreatedSecret_ShouldRemoveTokenFromFirstWrite(t *testing.T) {
	putCalls := 0
	getCalls := 0
	deleteCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			getCalls++
			if getCalls == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":["secret not found"]}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"new-token"},"metadata":{"version":1}}}`))
		case r.Method == http.MethodPut:
			var payload struct {
				Data map[string]interface{} `json:"data"`
			}
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &payload))
			putCalls++
			if putCalls == 1 {
				assert.Equal(t, "new-token", payload.Data["gitlab_token"])
			} else {
				assert.Empty(t, payload.Data)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"metadata":{"version":1}}}`))
		case r.Method == http.MethodDelete:
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatalf("unexpected vault request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.NoError(t, err)

	err = secret.DeleteCreatedSecret(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, getCalls)
	assert.Equal(t, 2, putCalls)
	assert.Equal(t, 0, deleteCalls)
}

func TestVaultSecret_DeleteCreatedSecret_ShouldNoopWhenSecretAlreadyExisted(t *testing.T) {
	deleteCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"old-token"},"metadata":{"version":1}}}`))
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"metadata":{"version":2}}}`))
		case r.Method == http.MethodDelete:
			deleteCalls++
			w.WriteHeader(http.StatusNoContent)
			t.Fatalf("unexpected vault delete call")
		default:
			t.Fatalf("unexpected vault request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.NoError(t, err)

	err = secret.DeleteCreatedSecret(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, deleteCalls)
}

func TestVaultSecret_DeleteCreatedSecret_ShouldSkipWhenTokenWasModified(t *testing.T) {
	getCalls := 0
	putCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			getCalls++
			if getCalls == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":["secret not found"]}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"attacker-token"},"metadata":{"version":1}}}`))
		case r.Method == http.MethodPut:
			putCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"metadata":{"version":1}}}`))
		default:
			t.Fatalf("unexpected vault request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.NoError(t, err)

	err = secret.DeleteCreatedSecret(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token key in vault secret")
	assert.Equal(t, 2, getCalls)
	assert.Equal(t, 1, putCalls)
}

func TestVaultSecret_DeleteCreatedSecret_ShouldFailOnCleanupCASConflict(t *testing.T) {
	getCalls := 0
	putCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			getCalls++
			if getCalls == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":["secret not found"]}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"new-token"},"metadata":{"version":1}}}`))
		case r.Method == http.MethodPut:
			putCalls++
			var payload struct {
				Data map[string]interface{} `json:"data"`
			}
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &payload))
			if putCalls == 1 {
				assert.Equal(t, "new-token", payload.Data["gitlab_token"])
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{"metadata":{"version":1}}}`))
				return
			}
			assert.Empty(t, payload.Data)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"errors":["check-and-set parameter did not match the current version"]}`))
		default:
			t.Fatalf("unexpected vault request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.NoError(t, err)

	err = secret.DeleteCreatedSecret(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing token key from vault secret")
	assert.Equal(t, 2, getCalls)
	assert.Equal(t, 2, putCalls)
}

func TestVaultSecret_DeleteCreatedSecret_ShouldFailOnCleanupPermissionDenied(t *testing.T) {
	getCalls := 0
	putCalls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			getCalls++
			if getCalls == 1 {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"errors":["secret not found"]}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"data":{"gitlab_token":"new-token"},"metadata":{"version":1}}}`))
		case r.Method == http.MethodPut:
			putCalls++
			if putCalls == 1 {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"data":{"metadata":{"version":1}}}`))
				return
			}
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":["permission denied"]}`))
		default:
			t.Fatalf("unexpected vault request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer ts.Close()

	client, err := vault.NewClient(&vault.Config{Address: ts.URL})
	require.NoError(t, err)

	secret := &VaultSecret{Path: "gitlab/project", Key: "gitlab_token", MountPath: "kv", Client: client}
	err = secret.Write(context.Background(), "new-token")
	require.NoError(t, err)

	err = secret.DeleteCreatedSecret(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "removing token key from vault secret")
	assert.Equal(t, 2, getCalls)
	assert.Equal(t, 2, putCalls)
}
