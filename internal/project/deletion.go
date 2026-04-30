package project

import (
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/logger"
	"github.com/nabsku/token-tumbler/internal/types/repository"
	"time"

	"go.uber.org/zap"
	"gitlab.com/gitlab-org/api/client-go"
)

func DeleteProjectTokens(gitlabClient *gitlab.Client, repo *repository.Repository, prefix string) error {
	l := logger.GetLogger()

	l.Info("checking for old tokens", zap.String("repo", *repo.RepoName))

	project, err := GatherProject(gitlabClient, repo)
	if err != nil {
		l.Error(fmt.Errorf("error fetching project %s: %v", *repo.RepoName, err).Error())
		return err
	}
	if project == nil {
		return fmt.Errorf("no projects found for %s", *repo.RepoName)
	}

	tokens, err := GatherProjectTokenInfo(gitlabClient, project.ID)
	if err != nil {
		l.Error(fmt.Errorf("error fetching project tokens for %s: %v", *repo.RepoName, err).Error())
		return err
	}

	var prefixedTokens []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		if token.Revoked || !token.Active {
			continue
		}
		if parseOk, errTokenParse := repo.ParseTokenName(prefix, token.Name); parseOk {
			prefixedTokens = append(prefixedTokens, token)
		} else if errTokenParse != nil {
			l.Debug(fmt.Errorf("error parsing token name for %s: %v", token.Name, errTokenParse).Error())
		}
	}

	if len(prefixedTokens) <= 1 {
		l.Debug("only one token found, not revoking", zap.String("repo", *repo.RepoName))
		return nil
	}

	var newestToken *gitlab.ProjectAccessToken
	for _, token := range prefixedTokens {
		if token.CreatedAt == nil {
			l.Debug("token has no creation date, skipping as newest candidate", zap.String("token_name", token.Name))
			continue
		}
		if newestToken == nil || token.CreatedAt.After(*newestToken.CreatedAt) {
			newestToken = token
		}
	}
	if newestToken == nil {
		l.Debug("no dated token found, not revoking", zap.String("repo", *repo.RepoName))
		return nil
	}

	var revokeErr error
	for _, token := range prefixedTokens {
		l.Debug("checking token for deletion", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))
		shouldDelete := checkProjectTokenDeletion(repo, token, newestToken)

		if shouldDelete {
			l.Debug("deleting token", zap.String("token_name", token.Name), zap.String("repo", *repo.RepoName))
			_, err := gitlabClient.ProjectAccessTokens.RevokeProjectAccessToken(project.ID, token.ID)
			if err != nil {
				l.Error(fmt.Errorf("error deleting token %s: %v", token.Name, err).Error())
				revokeErr = errors.Join(revokeErr, fmt.Errorf("deleting token %s from project %s: %w", token.Name, *repo.RepoName, err))
			} else {
				l.Info("deleted token", zap.String("token_name", token.Name), zap.String("repo", *repo.RepoName))
			}
		}
	}
	return revokeErr
}

func checkProjectTokenDeletion(entry *repository.Repository, token *gitlab.ProjectAccessToken, newestToken *gitlab.ProjectAccessToken) bool {
	l := logger.GetLogger()

	l.Debug("checking token for deletion", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))
	if token.CreatedAt == nil || newestToken == nil || newestToken.CreatedAt == nil || entry.GracePeriod == nil {
		l.Debug("missing token creation date or grace period, not deleting")
		return false
	}
	l.Debug("token creation date", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID), zap.Time("created_at", *token.CreatedAt))
	l.Debug("newest token date", zap.Time("created_at", *newestToken.CreatedAt))

	if token.ID == newestToken.ID {
		l.Debug("Token is the newest, not deleting")
		return false
	}

	l.Debug("checking if token is older than grace period", zap.String("token_name", token.Name), zap.Int64("token_id", token.ID))

	if time.Now().After(newestToken.CreatedAt.Add(entry.GracePeriod.ToDuration())) {
		return true
	}

	return false
}
