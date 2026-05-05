package secrets

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sSecret struct {
	Namespace              string
	SecretName             string
	SecretKey              string
	Client                 kubernetes.Interface
	createdOnWrite         bool
	createdResourceVersion string
	createdTokenValue      string
}

func (ks *K8sSecret) InitClient(ctx context.Context) error {
	if ks.Client != nil {
		return nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			configOverrides,
		)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return fmt.Errorf("unable to load kubernetes config: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("unable to create kubernetes client: %w", err)
	}

	ks.Client = client
	return nil
}

func (ks *K8sSecret) Read(ctx context.Context) (string, error) {
	err := ks.InitClient(ctx)
	if err != nil {
		return "", fmt.Errorf("initializing kubernetes client: %w", err)
	}

	namespace := strings.TrimSpace(ks.Namespace)
	secretName := strings.TrimSpace(ks.SecretName)
	secretKey := strings.TrimSpace(ks.SecretKey)

	if namespace == "" {
		return "", fmt.Errorf("k8sNamespace must not be blank")
	}
	if secretName == "" {
		return "", fmt.Errorf("k8sSecretName must not be blank")
	}
	if secretKey == "" {
		return "", fmt.Errorf("k8sSecretKey must not be blank")
	}

	secret, err := ks.Client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("reading kubernetes secret %s/%s: %w", namespace, secretName, err)
	}

	value, ok := secret.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("kubernetes secret %s/%s does not contain key %s", namespace, secretName, secretKey)
	}

	return string(value), nil
}

func (ks *K8sSecret) Write(ctx context.Context, value string) error {
	ks.createdOnWrite = false
	ks.createdResourceVersion = ""
	ks.createdTokenValue = ""

	err := ks.InitClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing kubernetes client: %w", err)
	}

	namespace := strings.TrimSpace(ks.Namespace)
	secretName := strings.TrimSpace(ks.SecretName)
	secretKey := strings.TrimSpace(ks.SecretKey)

	if namespace == "" {
		return fmt.Errorf("k8sNamespace must not be blank")
	}
	if secretName == "" {
		return fmt.Errorf("k8sSecretName must not be blank")
	}
	if secretKey == "" {
		return fmt.Errorf("k8sSecretKey must not be blank")
	}

	secret, err := ks.Client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					secretKey: []byte(value),
				},
			}
			createdSecret, err := ks.Client.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating kubernetes secret %s/%s: %w", namespace, secretName, err)
			}
			ks.createdOnWrite = true
			ks.createdResourceVersion = createdSecret.ResourceVersion
			ks.createdTokenValue = value
			return nil
		}
		return fmt.Errorf("reading kubernetes secret %s/%s: %w", namespace, secretName, err)
	}
	ks.createdOnWrite = false
	ks.createdResourceVersion = ""
	ks.createdTokenValue = ""

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[secretKey] = []byte(value)

	_, err = ks.Client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating kubernetes secret %s/%s: %w", namespace, secretName, err)
	}
	ks.createdOnWrite = false

	return nil
}

func (ks *K8sSecret) DeleteCreatedSecret(ctx context.Context) error {
	namespace := strings.TrimSpace(ks.Namespace)
	secretName := strings.TrimSpace(ks.SecretName)
	if namespace == "" {
		return fmt.Errorf("k8sNamespace must not be blank")
	}
	if secretName == "" {
		return fmt.Errorf("k8sSecretName must not be blank")
	}
	if !ks.createdOnWrite {
		return nil
	}

	err := ks.InitClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing kubernetes client: %w", err)
	}

	secret, err := ks.Client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			ks.createdOnWrite = false
			ks.createdResourceVersion = ""
			ks.createdTokenValue = ""
			return nil
		}
		return fmt.Errorf("reading kubernetes secret %s/%s for cleanup: %w", namespace, secretName, err)
	}

	if ks.createdResourceVersion != "" && secret.ResourceVersion != ks.createdResourceVersion {
		ks.createdOnWrite = false
		ks.createdResourceVersion = ""
		ks.createdTokenValue = ""
		return fmt.Errorf("kubernetes secret %s/%s changed since token was written; skipping rollback token key cleanup", namespace, secretName)
	}
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	if current, ok := secret.Data[ks.SecretKey]; !ok || ks.createdTokenValue != string(current) {
		ks.createdOnWrite = false
		ks.createdResourceVersion = ""
		ks.createdTokenValue = ""
		return fmt.Errorf("token key in kubernetes secret %s/%s was modified after write; skipping rollback token key cleanup", namespace, secretName)
	}

	delete(secret.Data, ks.SecretKey)

	_, err = ks.Client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating kubernetes secret %s/%s: %w", namespace, secretName, err)
	}
	ks.createdOnWrite = false
	ks.createdResourceVersion = ""
	ks.createdTokenValue = ""
	return nil
}

func (ks *K8sSecret) metaKey() string {
	return ks.SecretKey + "-meta"
}

func (ks *K8sSecret) ReadMetadata(ctx context.Context) (TokenMetadata, error) {
	err := ks.InitClient(ctx)
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("initializing kubernetes client: %w", err)
	}

	namespace := strings.TrimSpace(ks.Namespace)
	secretName := strings.TrimSpace(ks.SecretName)
	metaKey := strings.TrimSpace(ks.metaKey())

	if namespace == "" {
		return TokenMetadata{}, fmt.Errorf("k8sNamespace must not be blank")
	}
	if secretName == "" {
		return TokenMetadata{}, fmt.Errorf("k8sSecretName must not be blank")
	}

	secret, err := ks.Client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return TokenMetadata{}, nil
		}
		return TokenMetadata{}, fmt.Errorf("reading kubernetes secret %s/%s: %w", namespace, secretName, err)
	}

	data, ok := secret.Data[metaKey]
	if !ok {
		return TokenMetadata{}, nil
	}

	meta, err := parseTokenMetadata(string(data))
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("parsing kubernetes metadata %s/%s key %s: %w", namespace, secretName, metaKey, err)
	}
	return meta, nil
}

func (ks *K8sSecret) WriteMetadata(ctx context.Context, meta TokenMetadata) error {
	err := ks.InitClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing kubernetes client: %w", err)
	}

	namespace := strings.TrimSpace(ks.Namespace)
	secretName := strings.TrimSpace(ks.SecretName)
	metaKey := strings.TrimSpace(ks.metaKey())

	if namespace == "" {
		return fmt.Errorf("k8sNamespace must not be blank")
	}
	if secretName == "" {
		return fmt.Errorf("k8sSecretName must not be blank")
	}

	data, err := formatTokenMetadata(meta)
	if err != nil {
		return fmt.Errorf("formatting kubernetes metadata: %w", err)
	}

	secret, err := ks.Client.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			newSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					metaKey: []byte(data),
				},
			}
			_, err = ks.Client.CoreV1().Secrets(namespace).Create(ctx, newSecret, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("creating kubernetes secret %s/%s: %w", namespace, secretName, err)
			}
			return nil
		}
		return fmt.Errorf("reading kubernetes secret %s/%s: %w", namespace, secretName, err)
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[metaKey] = []byte(data)

	_, err = ks.Client.CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("updating kubernetes secret %s/%s: %w", namespace, secretName, err)
	}
	return nil
}
