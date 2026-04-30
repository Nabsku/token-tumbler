package project

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

var ErrInvalidProjectTokenResponse = errors.New("invalid project access token response")

func CreateNewProjectToken(gitlabClient *gitlab.Client, projectID int64, entry *repository.Repository, prefix string) (*gitlab.ProjectAccessToken, error) {
	l := logger.GetLogger()

	l.Debug("creating new project token", zap.String("repo", *entry.RepoName))
	expiryDate, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}

	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}

	token, err := createPATokenWithTokenOptions(gitlabClient, projectID, tokenName, entry.Permissions, expiryDate)

	if err != nil {
		return nil, err
	}
	if err := validateProjectAccessTokenResponse(token); err != nil {
		return nil, err
	}

	return token, nil
}

func createPATokenWithTokenOptions(gitlabClient *gitlab.Client, projectID int64, name string, permissions []string, t *time.Time) (*gitlab.ProjectAccessToken, error) {
	options := createProjectAccessTokenOptions(name, permissions, t)

	token, _, err := gitlabClient.ProjectAccessTokens.CreateProjectAccessToken(projectID, options)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func RenewProjectAccessToken(gitlabClient *gitlab.Client, projectID int64, entry *repository.Repository, prefix string) (*gitlab.ProjectAccessToken, error) {
	tokenName, err := entry.NewTokenName(prefix)
	if err != nil {
		return nil, err
	}
	expiryDate, err := entry.GetExpiryDate()
	if err != nil {
		return nil, err
	}

	token, err := createPATokenWithTokenOptions(gitlabClient, projectID, tokenName, entry.Permissions, expiryDate)

	if err != nil {
		return nil, err
	}
	if err := validateProjectAccessTokenResponse(token); err != nil {
		return nil, err
	}

	return token, nil
}

func validateProjectAccessTokenResponse(token *gitlab.ProjectAccessToken) error {
	if token == nil {
		return fmt.Errorf("%w: token is nil", ErrInvalidProjectTokenResponse)
	}
	if token.ID == 0 {
		return fmt.Errorf("%w: token ID is empty", ErrInvalidProjectTokenResponse)
	}
	if strings.TrimSpace(token.Token) == "" {
		return fmt.Errorf("%w: token value is empty", ErrInvalidProjectTokenResponse)
	}
	return nil
}

func createProjectAccessTokenOptions(tokenName string, scopes []string, expiry *time.Time) *gitlab.CreateProjectAccessTokenOptions {
	return &gitlab.CreateProjectAccessTokenOptions{
		Name:      &tokenName,
		Scopes:    &scopes,
		ExpiresAt: (*gitlab.ISOTime)(expiry),
	}
}
