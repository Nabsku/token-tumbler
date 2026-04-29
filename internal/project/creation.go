package project

import (
	"fmt"
	"github.com/nabsku/token-chaser/internal/logger"
	"github.com/nabsku/token-chaser/internal/types/repository"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
)

func CreateNewProjectToken(gitlabClient *gitlab.Client, projectID int, entry *repository.Repository, prefix string) (*gitlab.ProjectAccessToken, error) {
	l := logger.GetLogger()

	l.Debug(fmt.Sprintf("Creating new project token for %s", *entry.RepoName))
	expireAtInDaysCheck, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}
	expireAtInDays := expireAtInDaysCheck

	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}

	token, err := createPATokenWithTokenOptions(gitlabClient, projectID, tokenName, entry.Permissions, expireAtInDays)

	if err != nil {
		return nil, err
	}

	return token, nil
}

func createPATokenWithTokenOptions(gitlabClient *gitlab.Client, projectID int, name string, permissions []string, t *time.Time) (*gitlab.ProjectAccessToken, error) {
	options := createProjectAccessTokenOptions(name, permissions, t)

	token, _, err := gitlabClient.ProjectAccessTokens.CreateProjectAccessToken(projectID, options)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func RenewProjectAccessToken(gitlabClient *gitlab.Client, projectID int, entry *repository.Repository, prefix string) (*gitlab.ProjectAccessToken, error) {
	tokenName, _ := entry.NewTokenName(prefix)
	expiryDate, _ := entry.GetExpiryDate()

	token, err := createPATokenWithTokenOptions(gitlabClient, projectID, tokenName, entry.Permissions, expiryDate)

	if err != nil {
		return nil, err
	}

	return token, nil
}

func createProjectAccessTokenOptions(tokenName string, scopes []string, expiry *time.Time) *gitlab.CreateProjectAccessTokenOptions {
	return &gitlab.CreateProjectAccessTokenOptions{
		Name:      &tokenName,
		Scopes:    &scopes,
		ExpiresAt: (*gitlab.ISOTime)(expiry),
	}
}
