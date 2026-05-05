package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/nabsku/token-tumbler/internal/types/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

var _ SecretStore = (*K8sSecret)(nil)

func TestForRepository_ShouldRejectBlankK8sConfig(t *testing.T) {
	namespace := "  "
	secretName := "my-secret"
	secretKey := "token"
	entry := &repository.Repository{
		SecretStore:   "k8s",
		K8sNamespace:  &namespace,
		K8sSecretName: &secretName,
		K8sSecretKey:  &secretKey,
	}

	store, err := ForRepository(entry)

	require.Error(t, err)
	assert.True(t, errors.Is(err, repository.ErrInvalidRepositoryConfig))
	assert.Contains(t, err.Error(), "must not be blank")
	assert.Nil(t, store)
}

func TestForRepository_ShouldTrimK8sConfig(t *testing.T) {
	namespace := "  default  "
	secretName := "  my-secret  "
	secretKey := "  token  "
	entry := &repository.Repository{
		SecretStore:   "k8s",
		K8sNamespace:  &namespace,
		K8sSecretName: &secretName,
		K8sSecretKey:  &secretKey,
	}

	store, err := ForRepository(entry)

	require.NoError(t, err)
	secret, ok := store.(*K8sSecret)
	require.True(t, ok)
	assert.Equal(t, "default", secret.Namespace)
	assert.Equal(t, "my-secret", secret.SecretName)
	assert.Equal(t, "token", secret.SecretKey)
}

func TestK8sSecret_Read_ShouldReturnValue(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("my-token-value"),
		},
	})

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	value, err := secret.Read(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "my-token-value", value)
}

func TestK8sSecret_Read_ShouldReturnErrorForMissingKey(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"other": []byte("value"),
		},
	})

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	_, err := secret.Read(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain key")
}

func TestK8sSecret_Read_ShouldReturnErrorForMissingSecret(t *testing.T) {
	client := fake.NewSimpleClientset()

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	_, err := secret.Read(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading kubernetes secret")
}

func TestK8sSecret_Write_ShouldCreateNewSecret(t *testing.T) {
	client := fake.NewSimpleClientset()

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	err := secret.Write(context.Background(), "new-token-value")
	require.NoError(t, err)

	created, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("new-token-value"), created.Data["token"])
}

func TestK8sSecret_DeleteCreatedSecret_ShouldDeleteCreatedSecret(t *testing.T) {
	client := fake.NewSimpleClientset()

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	require.NoError(t, secret.Write(context.Background(), "new-token-value"))

	err := secret.DeleteCreatedSecret(context.Background())
	require.NoError(t, err)

	created, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	_, tokenPresent := created.Data["token"]
	assert.False(t, tokenPresent)
	assert.True(t, len(created.Data) == 0)
}

func TestK8sSecret_DeleteCreatedSecret_ShouldNoopOnExistingSecret(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("old-token"),
			"other": []byte("preserve-me"),
		},
	})

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	require.NoError(t, secret.Write(context.Background(), "new-token-value"))
	err := secret.DeleteCreatedSecret(context.Background())
	require.NoError(t, err)

	updated, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("new-token-value"), updated.Data["token"])
	assert.Equal(t, []byte("preserve-me"), updated.Data["other"])
}

func TestK8sSecret_DeleteCreatedSecret_ShouldSkipWhenSecretWasModifiedByAnotherActor(t *testing.T) {
	client := fake.NewSimpleClientset()

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	require.NoError(t, secret.Write(context.Background(), "new-token"))

	existing, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	secret.createdResourceVersion = "1"
	existing.ResourceVersion = "2"
	existing.Data["token"] = []byte("attacker-token")
	updated, err := client.CoreV1().Secrets("default").Update(context.Background(), existing, metav1.UpdateOptions{})
	require.NoError(t, err)

	err = secret.DeleteCreatedSecret(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "changed since token was written")

	current, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("attacker-token"), current.Data["token"])
	assert.Equal(t, updated.ResourceVersion, current.ResourceVersion)
}

func TestK8sSecret_DeleteCreatedSecret_ShouldSkipWhenTokenWasModifiedButResourceVersionCleared(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "my-secret",
			Namespace:       "default",
			ResourceVersion: "123",
		},
		Data: map[string][]byte{
			"token": []byte("attacker-token"),
		},
	})

	secret := &K8sSecret{
		Namespace:              "default",
		SecretName:             "my-secret",
		SecretKey:              "token",
		Client:                 client,
		createdOnWrite:         true,
		createdTokenValue:      "expected-token",
		createdResourceVersion: "",
	}

	err := secret.DeleteCreatedSecret(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token key in kubernetes secret")

	current, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("attacker-token"), current.Data["token"])
}

func TestK8sSecret_DeleteCreatedSecret_ShouldReturnErrorWhenUpdateIsForbidden(t *testing.T) {
	client := fake.NewSimpleClientset()

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	require.NoError(t, secret.Write(context.Background(), "new-token"))

	client.Fake.PrependReactor("update", "secrets", func(_ k8sTesting.Action) (bool, runtime.Object, error) {
		return true, nil, &k8serrors.StatusError{ErrStatus: metav1.Status{
			Status:  "Failure",
			Code:    403,
			Reason:  metav1.StatusReasonForbidden,
			Message: "forbidden",
		}}
	})

	err := secret.DeleteCreatedSecret(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updating kubernetes secret")
}

func TestK8sSecret_Write_ShouldUpdateExistingSecret(t *testing.T) {
	client := fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"token": []byte("old-value"),
			"other": []byte("preserve-me"),
		},
	})

	secret := &K8sSecret{
		Namespace:  "default",
		SecretName: "my-secret",
		SecretKey:  "token",
		Client:     client,
	}

	err := secret.Write(context.Background(), "new-token-value")
	require.NoError(t, err)

	updated, err := client.CoreV1().Secrets("default").Get(context.Background(), "my-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []byte("new-token-value"), updated.Data["token"])
	assert.Equal(t, []byte("preserve-me"), updated.Data["other"])
}

func TestK8sSecret_Read_ShouldReturnErrorForBlankNamespace(t *testing.T) {
	secret := &K8sSecret{Namespace: "  ", SecretName: "my-secret", SecretKey: "token", Client: fake.NewSimpleClientset()}
	_, err := secret.Read(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8sNamespace must not be blank")
}

func TestK8sSecret_Read_ShouldReturnErrorForBlankSecretName(t *testing.T) {
	secret := &K8sSecret{Namespace: "default", SecretName: "  ", SecretKey: "token", Client: fake.NewSimpleClientset()}
	_, err := secret.Read(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8sSecretName must not be blank")
}

func TestK8sSecret_Read_ShouldReturnErrorForBlankSecretKey(t *testing.T) {
	secret := &K8sSecret{Namespace: "default", SecretName: "my-secret", SecretKey: "  ", Client: fake.NewSimpleClientset()}
	_, err := secret.Read(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8sSecretKey must not be blank")
}

func TestK8sSecret_Write_ShouldReturnErrorForBlankNamespace(t *testing.T) {
	secret := &K8sSecret{Namespace: "  ", SecretName: "my-secret", SecretKey: "token", Client: fake.NewSimpleClientset()}
	err := secret.Write(context.Background(), "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8sNamespace must not be blank")
}

func TestK8sSecret_Write_ShouldReturnErrorForBlankSecretName(t *testing.T) {
	secret := &K8sSecret{Namespace: "default", SecretName: "  ", SecretKey: "token", Client: fake.NewSimpleClientset()}
	err := secret.Write(context.Background(), "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8sSecretName must not be blank")
}

func TestK8sSecret_Write_ShouldReturnErrorForBlankSecretKey(t *testing.T) {
	secret := &K8sSecret{Namespace: "default", SecretName: "my-secret", SecretKey: "  ", Client: fake.NewSimpleClientset()}
	err := secret.Write(context.Background(), "token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "k8sSecretKey must not be blank")
}
