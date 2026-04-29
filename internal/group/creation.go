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

	expireAtInDaysCheck, err := entry.GetExpiryDate()
	l.Debug(fmt.Sprintf("ExpireAtInDaysCheck: %v", expireAtInDaysCheck))

	if err != nil {
		return nil, err
	}
	expireAtInDays := expireAtInDaysCheck

	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}

	opts := createGroupAccessTokenOptions(tokenName, entry.Permissions, expireAtInDays)

	token, _, err := gitlabClient.GroupAccessTokens.CreateGroupAccessToken(groupID, opts)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func RenewGroupAccessToken(gitlabClient *gitlab.Client, groupID int, entry *repository.Repository, prefix string) (*gitlab.GroupAccessToken, error) {
	tokenName, _ := entry.NewTokenName(prefix)
	expiryDate, _ := entry.GetExpiryDate()
	opts := createGroupAccessTokenOptions(tokenName, entry.Permissions, expiryDate)

	token, _, err := gitlabClient.GroupAccessTokens.CreateGroupAccessToken(groupID, opts)
	if err != nil {
		return &gitlab.GroupAccessToken{}, err
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
