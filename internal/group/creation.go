package group

import (
	"fmt"
	"github.com/nabsku/token-chaser/internal/logger"
	"github.com/nabsku/token-chaser/internal/types/repository"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
)

func CreateNewGroupToken(gitlabClient *gitlab.Client, groupID int, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
	l := logger.GetLogger()

	expiryDate, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}
	l.Debug(fmt.Sprintf("ExpiryDate: %v", expiryDate))

	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}
	return createGroupAccessToken(gitlabClient, groupID, tokenName, entry.Permissions, expiryDate)
}

func RenewGroupAccessToken(gitlabClient *gitlab.Client, groupID int, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
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
	return token, nil
}

func createGroupAccessToken(gitlabClient *gitlab.Client, groupID int, tokenName string, scopes []string, expiry *time.Time) (*gitlab.GroupAccessToken, error) {
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
