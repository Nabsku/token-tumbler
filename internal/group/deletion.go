package group

import (
	"fmt"
	"github.com/nabsku/token-chaser/internal/logger"
	"github.com/nabsku/token-chaser/internal/types/repository"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
)

func DeleteGroupTokens(gitlabClient *gitlab.Client, repo *repository.Repository, prefix string) error {
	l := logger.GetLogger()

	l.Debug(fmt.Sprintf("Checking for old tokens in repo %s", *repo.GroupName))

	group, err := GatherGroup(gitlabClient, repo)
	if err != nil {
		return err
	}

	tokens, err := GatherGroupTokenInfoByPrefix(gitlabClient, group.ID, prefix, *repo)
	l.Debug(fmt.Sprintf("Found %v for %s", tokens, *repo.GroupName))
	if err != nil {
		return err
	}

	if len(tokens) <= 1 {
		l.Debug(fmt.Sprintf("Only found 1 token for %s, not revoking", *repo.GroupName))
		return nil
	}

	var newestToken gitlab.GroupAccessToken
	for _, token := range tokens {
		if newestToken.CreatedAt == nil {
			newestToken = *token
		}
		if token.CreatedAt.After(*newestToken.CreatedAt) {
			newestToken = *token
		}
	}

	for _, token := range tokens {
		l.Debug(fmt.Sprintf("Parsing token %s before deletion", token.Name))
		if parseOk, errTokenParse := repo.ParseTokenName(prefix, token.Name); parseOk {
			l.Debug(fmt.Sprintf("Checking token %s for deletion", token.Name))
			shouldDelete := checkGroupTokenDeletion(repo, token, &newestToken)

			if shouldDelete {
				l.Debug(fmt.Sprintf("Deleting token %s from repo %s", token.Name, *repo.GroupName))
				_, err := gitlabClient.GroupAccessTokens.RevokeGroupAccessToken(group.ID, token.ID)
				if err != nil {
					l.Error(fmt.Errorf("error deleting token %s: %v", token.Name, err).Error())
				} else {
					l.Info(fmt.Sprintf("Deleted token %s from repo %s", token.Name, *repo.GroupName))
				}
			}
		} else if errTokenParse != nil {
			l.Error(fmt.Errorf("error parsing token name for %s: %v", token.Name, errTokenParse).Error())
		}
	}
	return nil
}

func checkGroupTokenDeletion(entry *repository.Repository, token *gitlab.GroupAccessToken, newestToken *gitlab.GroupAccessToken) bool {
	l := logger.GetLogger()

	l.Debug(fmt.Sprintf("Checking token for deletion: %s", token.Token))
	l.Debug(fmt.Sprintf("Token created at: %s", token.CreatedAt))
	l.Debug(fmt.Sprintf("Newest Token is: %v", newestToken.CreatedAt))

	if token.ID == newestToken.ID {
		l.Debug("Token is the newest token, not deleting")
		return false
	}

	l.Debug(fmt.Sprintf("Checking if token %s is older than grace period", token.Token))

	if time.Now().After(newestToken.CreatedAt.Add(entry.GracePeriod.ToDuration())) {
		return true
	}

	return false
}
