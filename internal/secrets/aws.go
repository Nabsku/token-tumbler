package secrets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type AWSSecret struct {
	SecretName string
	Region     string
	Client     *secretsmanager.Client
}

func (as *AWSSecret) InitClient(ctx context.Context) error {
	region := strings.TrimSpace(as.Region)
	if region == "" {
		return fmt.Errorf("awsRegion must not be blank")
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("loading AWS config for region %s: %w", region, err)
	}

	as.Client = secretsmanager.NewFromConfig(cfg)
	return nil
}

func (as *AWSSecret) Read(ctx context.Context) (string, error) {
	err := as.InitClient(ctx)
	if err != nil {
		return "", fmt.Errorf("initializing AWS client: %w", err)
	}

	secretName := strings.TrimSpace(as.SecretName)
	if secretName == "" {
		return "", fmt.Errorf("awsSecretName must not be blank")
	}

	result, err := as.Client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretName),
	})
	if err != nil {
		return "", fmt.Errorf("reading AWS secret %s: %w", secretName, err)
	}

	if result.SecretString == nil {
		return "", fmt.Errorf("AWS secret %s has no string value", secretName)
	}

	return *result.SecretString, nil
}

func (as *AWSSecret) Write(ctx context.Context, value string) error {
	err := as.InitClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing AWS client: %w", err)
	}

	secretName := strings.TrimSpace(as.SecretName)
	if secretName == "" {
		return fmt.Errorf("awsSecretName must not be blank")
	}

	_, err = as.Client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretName),
		SecretString: aws.String(value),
	})
	if err != nil {
		return fmt.Errorf("writing AWS secret %s: %w", secretName, err)
	}

	return nil
}

func (as *AWSSecret) metaSecretName() string {
	return as.SecretName + "-meta"
}

func (as *AWSSecret) ReadMetadata(ctx context.Context) (TokenMetadata, error) {
	err := as.InitClient(ctx)
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("initializing AWS client: %w", err)
	}

	metaName := strings.TrimSpace(as.metaSecretName())
	if metaName == "" {
		return TokenMetadata{}, fmt.Errorf("awsSecretName must not be blank")
	}

	result, err := as.Client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(metaName),
	})
	if err != nil {
		var notFound *types.ResourceNotFoundException
		if errors.As(err, &notFound) {
			return TokenMetadata{}, nil
		}
		return TokenMetadata{}, fmt.Errorf("reading AWS metadata secret %s: %w", metaName, err)
	}

	if result.SecretString == nil {
		return TokenMetadata{}, nil
	}

	meta, err := parseTokenMetadata(*result.SecretString)
	if err != nil {
		return TokenMetadata{}, fmt.Errorf("parsing AWS metadata secret %s: %w", metaName, err)
	}
	return meta, nil
}

func (as *AWSSecret) WriteMetadata(ctx context.Context, meta TokenMetadata) error {
	err := as.InitClient(ctx)
	if err != nil {
		return fmt.Errorf("initializing AWS client: %w", err)
	}

	metaName := strings.TrimSpace(as.metaSecretName())
	if metaName == "" {
		return fmt.Errorf("awsSecretName must not be blank")
	}

	data, err := formatTokenMetadata(meta)
	if err != nil {
		return fmt.Errorf("formatting AWS metadata: %w", err)
	}

	_, err = as.Client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(metaName),
		SecretString: aws.String(data),
	})
	if err != nil {
		return fmt.Errorf("writing AWS metadata secret %s: %w", metaName, err)
	}
	return nil
}
