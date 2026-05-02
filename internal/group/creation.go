package group

import (
	"context"
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"strings"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
	"go.uber.org/zap"
)

var ErrInvalidGroupTokenResponse = errors.New("invalid group access token response")

func CreateNewGroupToken(ctx context.Context, gitlabClient *gitlab.Client, groupID int64, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
	l := logger.GetLogger()

	expiryDate, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}
	l.Debug("computed token expiry", zap.Time("expiry", *expiryDate))

	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}
	token, err := createGroupAccessToken(ctx, gitlabClient, groupID, tokenName, entry.Permissions, entry.GitLabAccessLevel(), expiryDate)
	if err != nil {
		return nil, err
	}
	if err := validateGroupAccessTokenResponse(token); err != nil {
		return nil, err
	}
	return token, nil
}

func RenewGroupAccessToken(ctx context.Context, gitlabClient *gitlab.Client, groupID int64, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}
	expiryDate, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}
	token, err := createGroupAccessToken(ctx, gitlabClient, groupID, tokenName, entry.Permissions, entry.GitLabAccessLevel(), expiryDate)
	if err != nil {
		return nil, err
	}
	if err := validateGroupAccessTokenResponse(token); err != nil {
		return nil, err
	}
	return token, nil
}

func validateGroupAccessTokenResponse(token *gitlab.GroupAccessToken) error {
	if token == nil {
		return fmt.Errorf("%w: token is nil", ErrInvalidGroupTokenResponse)
	}
	if token.ID == 0 {
		return fmt.Errorf("%w: token ID is empty", ErrInvalidGroupTokenResponse)
	}
	if strings.TrimSpace(token.Token) == "" {
		return fmt.Errorf("%w: token value is empty", ErrInvalidGroupTokenResponse)
	}
	return nil
}

func createGroupAccessToken(ctx context.Context, gitlabClient *gitlab.Client, groupID int64, tokenName string, scopes []string, accessLevel *gitlab.AccessLevelValue, expiry *time.Time) (*gitlab.GroupAccessToken, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	opts := createGroupAccessTokenOptions(tokenName, scopes, accessLevel, expiry)
	token, _, err := gitlabClient.GroupAccessTokens.CreateGroupAccessToken(groupID, opts, gitlab.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	return token, nil
}

func createGroupAccessTokenOptions(tokenName string, scopes []string, accessLevel *gitlab.AccessLevelValue, expiry *time.Time) *gitlab.CreateGroupAccessTokenOptions {
	options := &gitlab.CreateGroupAccessTokenOptions{
		Name:      &tokenName,
		Scopes:    &scopes,
		ExpiresAt: (*gitlab.ISOTime)(expiry),
	}
	if accessLevel != nil {
		options.AccessLevel = accessLevel
	}
	return options
}
