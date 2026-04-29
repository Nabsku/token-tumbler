package project

import (
	"errors"
	"fmt"
	"github.com/nabsku/token-chaser/internal/logger"
	"github.com/nabsku/token-chaser/internal/types/repository"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
)

func DeleteProjectTokens(gitlabClient *gitlab.Client, repo *repository.Repository, prefix string) error {
	l := logger.GetLogger()

	l.Info(fmt.Sprintf("Checking for old tokens in repo %s", *repo.RepoName))

	opts := &gitlab.ListProjectsOptions{
		Search: gitlab.Ptr(*repo.RepoName),
	}
	projects, _, err := gitlabClient.Projects.ListProjects(opts)
	if err != nil {
		l.Error(fmt.Errorf("error fetching project %s: %v", *repo.RepoName, err).Error())
		return err
	}
	if len(projects) == 0 {
		return fmt.Errorf("no projects found for %s", *repo.RepoName)
	}

	tokens, _, err := gitlabClient.ProjectAccessTokens.ListProjectAccessTokens(projects[0].ID, nil)
	if err != nil {
		l.Error(fmt.Errorf("error fetching project tokens for %s: %v", *repo.RepoName, err).Error())
		return err
	}

	var prefixedTokens []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		if parseOk, errTokenParse := repo.ParseTokenName(prefix, token.Name); parseOk {
			prefixedTokens = append(prefixedTokens, token)
		} else if errTokenParse != nil {
			l.Debug(fmt.Errorf("error parsing token name for %s: %v", token.Name, errTokenParse).Error())
		}
	}

	if len(prefixedTokens) <= 1 {
		l.Debug(fmt.Sprintf("Found 1 token for %s, not revoking", *repo.RepoName))
		return nil
	}

	var newestToken *gitlab.ProjectAccessToken
	for _, token := range prefixedTokens {
		if token.CreatedAt == nil {
			l.Debug(fmt.Sprintf("Token %s has no creation date, skipping as newest candidate", token.Name))
			continue
		}
		if newestToken == nil || token.CreatedAt.After(*newestToken.CreatedAt) {
			newestToken = token
		}
	}
	if newestToken == nil {
		l.Debug(fmt.Sprintf("No dated token found for %s, not revoking", *repo.RepoName))
		return nil
	}

	var revokeErr error
	for _, token := range prefixedTokens {
		l.Debug(fmt.Sprintf("Parsing token %s before deletion", token.Name))
		l.Debug(fmt.Sprintf("Checking token %s for deletion", token.Name))
		shouldDelete := checkProjectTokenDeletion(repo, token, newestToken)

		if shouldDelete {
			l.Debug(fmt.Sprintf("Deleting token %s from repo %s", token.Name, *repo.RepoName))
			_, err := gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(projects[0].ID, token.ID)
			if err != nil {
				l.Error(fmt.Errorf("error deleting token %s: %v", token.Name, err).Error())
				revokeErr = errors.Join(revokeErr, fmt.Errorf("deleting token %s from project %s: %w", token.Name, *repo.RepoName, err))
			} else {
				l.Info(fmt.Sprintf("Deleted token %s from repo %s", token.Name, *repo.RepoName))
			}
		}
	}
	return revokeErr
}

func checkProjectTokenDeletion(entry *repository.Repository, token *gitlab.ProjectAccessToken, newestToken *gitlab.ProjectAccessToken) bool {
	l := logger.GetLogger()

	l.Debug(fmt.Sprintf("Checking token for deletion: %s", token.Token))
	l.Debug(fmt.Sprintf("Token created at: %s", token.CreatedAt))
	if token.CreatedAt == nil || newestToken == nil || newestToken.CreatedAt == nil || entry.GracePeriod == nil {
		l.Debug("Missing token creation date or grace period, not deleting")
		return false
	}
	l.Debug(fmt.Sprintf("Newest Token is: %v", newestToken.CreatedAt))

	if token.ID == newestToken.ID {
		l.Debug("Token is the newest, not deleting")
		return false
	}

	l.Debug(fmt.Sprintf("Checking if token %s is older than grace period", token.Token))

	if time.Now().After(newestToken.CreatedAt.Add(entry.GracePeriod.ToDuration())) {
		return true
	}

	return false
}
