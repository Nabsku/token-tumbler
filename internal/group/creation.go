package group

import (
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"strings"
	"time"

	"go.uber.org/zap"
	"gitlab.com/gitlab-org/api/client-go"
)

var ErrInvalidGroupTokenResponse = errors.New("invalid group access token response")

func CreateNewGroupToken(gitlabClient *gitlab.Client, groupID int64, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
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
	token, err := createGroupAccessToken(gitlabClient, groupID, tokenName, entry.Permissions, expiryDate)
	if err != nil {
		return nil, err
	}
	if err := validateGroupAccessTokenResponse(token); err != nil {
		return nil, err
	}
	return token, nil
}

func RenewGroupAccessToken(gitlabClient *gitlab.Client, groupID int64, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}
	expiryDate, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}
	token, err := createGroupAccessToken(gitlabClient, groupID, tokenName, entry.Permissions, expiryDate)
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

func createGroupAccessToken(gitlabClient *gitlab.Client, groupID int64, tokenName string, scopes []string, expiry *time.Time) (*gitlab.GroupAccessToken, error) {
	opts := createGroupAccessTokenOptions(tokenName, scopes, expiry)
	token, _, err := gitlabClient.GroupAccessTokens.CreateGroupAccessToken(groupID, opts)
	if err != nil {
		return nil, err
	}
	return token, nil
}

func createGroupAccessTokenOptions(tokenName string, scopes []string, expiry *time.Time) *gitlab.CreateGroupAccessTokenOptions {
	return &gitlab.CreateGroupAccessTokenOptions{
		Name:      &tokenName,
		Scopes:    &scopes,
		ExpiresAt: (*gitlab.ISOTime)(expiry),
	}
}
