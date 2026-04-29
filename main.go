package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/nabsku/token-chaser/internal/config"
	"github.com/nabsku/token-chaser/internal/group"
	"github.com/nabsku/token-chaser/internal/helper"
	"github.com/nabsku/token-chaser/internal/logger"
	"github.com/nabsku/token-chaser/internal/project"
	"github.com/nabsku/token-chaser/internal/secrets"
	"github.com/nabsku/token-chaser/internal/types/repository"

	"log"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"gitlab.com/gitlab-org/api/client-go"

	"go.uber.org/zap"
)

var (
	ErrGroupAndRepoDefined = errors.New("you cannot define both a Repository and Group name. Choose one or the other")
)

const (
	delay              = time.Duration(2) * time.Second
	errorString string = "while processing %v at index %v, the following error occurred: %w"
)

func readConfig() (*repository.Config, error) {
	buff, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	repoConfig := repository.Config{}
	err = yaml.Unmarshal(buff, &repoConfig)
	if err != nil {
		return nil, err
	}

	return &repoConfig, nil
}

func NewClient() (*gitlab.Client, error) {
	newConfig := config.NewConfig()
	gitlabClient, err := gitlab.NewClient(newConfig.GitlabToken, gitlab.WithBaseURL(newConfig.GitlabUrl))
	if err != nil {
		return &gitlab.Client{}, err
	}

	return gitlabClient, nil
}

func checkEnvVars(vars ...string) error {
	var missingVars []string
	for _, v := range vars {
		if !helper.CheckAndGetEnvVar(v) {
			missingVars = append(missingVars, v)
		}
	}
	if len(missingVars) > 0 {
		return fmt.Errorf("missing the following environment variables: %v", strings.Join(missingVars, ", "))
	}
	return nil
}

func secretStoreForToken(entry *repository.Repository, token string) secrets.SecretStore {
	switch strings.ToLower(entry.SecretStore) {
	case "vault":
		return &secrets.VaultSecret{
			Path:      *entry.VaultPath,
			Key:       *entry.VaultKey,
			Value:     token,
			MountPath: *entry.Mount,
		}
	default:
		return nil
	}
}

func writeSecret(ctx context.Context, entry *repository.Repository, secret secrets.SecretStore) {
	if secret == nil {
		return
	}

	l := logger.GetLogger()
	l.Info(fmt.Sprintf("Writing Secret to selected secret store (%v).", entry.SecretStore))
	if err := secret.Write(ctx); err != nil {
		panic(err)
	}
}

func matchingGroupTokens(tokens []*gitlab.GroupAccessToken, entry *repository.Repository, prefix string, index int) []*gitlab.GroupAccessToken {
	l := logger.GetLogger()
	var matches []*gitlab.GroupAccessToken
	for _, token := range tokens {
		if ok, err := entry.ParseTokenName(prefix, token.Name); ok {
			matches = append(matches, token)
		} else if err != nil {
			l.Debug(fmt.Errorf(errorString, *entry.GroupName, index, err).Error())
			break
		}
	}
	return matches
}

func matchingProjectTokens(tokens []*gitlab.ProjectAccessToken, entry *repository.Repository, prefix string, index int) []*gitlab.ProjectAccessToken {
	l := logger.GetLogger()
	var matches []*gitlab.ProjectAccessToken
	for _, token := range tokens {
		if ok, err := entry.ParseTokenName(prefix, token.Name); ok {
			l.Info(fmt.Sprintf("Token %v is valid, appending to queue of tokens to check further", token.Name))
			matches = append(matches, token)
		} else if err != nil {
			l.Debug(fmt.Errorf(errorString, *entry.RepoName, index, err).Error())
			break
		}
	}
	return matches
}

func processGroupTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error occurred while processing group tokens: ", r)
		}
	}()

	l := logger.GetLogger()

	var groupToken *gitlab.GroupAccessToken

	info, err := group.GatherGroup(gitlabClient, entry)
	if err != nil {
		l.Error(fmt.Errorf(errorString, *entry.GroupName, index, err).Error())
		return
	}

	if info == nil {
		l.Error(fmt.Errorf("no group returned for %v, skipping", *entry.GroupName).Error())
		return
	}

	tokenInfo, err := group.GatherGroupTokenInfo(gitlabClient, info.ID)

	if errors.Is(err, group.ErrTooManyGroupsInSearch) {
		l.Error(fmt.Errorf(errorString, *entry.GroupName, index, group.ErrTooManyGroupsInSearch).Error())
		return
	} else if err != nil {
		l.Error(fmt.Errorf(errorString, *entry.GroupName, index, err).Error())
		return
	}

	tokenQueue := matchingGroupTokens(tokenInfo, entry, yamlConfig.Prefix, index)

	if len(tokenQueue) < 1 {
		l.Info(fmt.Sprintf("No token in group %v yet, we're free to create one as we please.", *entry.GroupName))
		token, errTokenCreation := group.CreateNewGroupToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			l.Error(fmt.Errorf(errorString, *entry.GroupName, index, errTokenCreation).Error())
		}
		groupToken = token
	}

	needsRenewal, err := group.CheckGroupTokensForRenewal(tokenQueue, entry)
	if err != nil {
		l.Error(fmt.Errorf(errorString, *entry.GroupName, index, err).Error())
		return
	}

	if needsRenewal {
		l.Info(fmt.Sprintf("Token for %v in Group %v is ready to be renewed.\n", entry.Name, *entry.GroupName))
		token, errRenewal := group.RenewGroupAccessToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			l.Error(fmt.Errorf(errorString, entry.Name, index, errRenewal).Error())
		}
		groupToken = token
	} else {
		l.Info(fmt.Sprintf("No tokens for %v in Group %v need renewal at this time.\n", entry.Name, *entry.GroupName))
	}

	if groupToken == nil {
		return
	}

	writeSecret(ctx, entry, secretStoreForToken(entry, groupToken.Token))
}

func processProjectTokens(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository, index int, yamlConfig *repository.Config) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error occurred while processing project tokens: ", r)
		}
	}()

	var projectToken *gitlab.ProjectAccessToken

	l := logger.GetLogger()

	info, err := project.GatherProject(gitlabClient, entry)
	if err != nil {
		l.Error(fmt.Errorf(errorString, *entry.RepoName, index, err).Error())
		return
	}

	if info == nil {
		l.Error(fmt.Errorf("no project returned for %v, skipping", *entry.RepoName).Error())
		return
	}

	tokenInfo, err := project.GatherProjectTokenInfo(gitlabClient, info.ID)

	if errors.Is(err, project.ErrTooManyProjectsInSearch) {
		l.Error(fmt.Errorf(errorString, *entry.RepoName, index, project.ErrTooManyProjectsInSearch).Error())
		return
	} else if err != nil {
		l.Error(fmt.Errorf(errorString, *entry.RepoName, index, err).Error())
		return
	}

	tokenQueue := matchingProjectTokens(tokenInfo, entry, yamlConfig.Prefix, index)

	if len(tokenQueue) < 1 {
		l.Info(fmt.Sprintf("No token yet for repo %v, we're free to create one as we please.", *entry.RepoName))

		token, errTokenCreation := project.CreateNewProjectToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errTokenCreation != nil {
			l.Error(fmt.Errorf(errorString, *entry.RepoName, index, errTokenCreation).Error())
			return
		}
		projectToken = token
	}

	needsRenewal, err := project.CheckProjectTokensForRenewal(tokenQueue, entry)
	if err != nil {
		l.Error(fmt.Errorf(errorString, *entry.RepoName, index, err).Error())
		return
	}

	if needsRenewal {
		l.Info(fmt.Sprintf("Token for %v in Repo %v is ready to be renewed.\n", entry.Name, *entry.RepoName))
		token, errRenewal := project.RenewProjectAccessToken(gitlabClient, info.ID, entry, yamlConfig.Prefix)
		if errRenewal != nil {
			l.Error(fmt.Errorf(errorString, entry.Name, index, errRenewal).Error())
		}
		projectToken = token
	} else {
		l.Info(fmt.Sprintf("No tokens for %v in Repo %v need renewal at this time.\n", entry.Name, *entry.RepoName))
	}

	if projectToken == nil {
		return
	}

	writeSecret(ctx, entry, secretStoreForToken(entry, projectToken.Token))
}

func main() {
	l := logger.GetLogger()

	if err := checkEnvVars("GITLAB_TOKEN", "GITLAB_URL", "APPROLE_ID", "APPROLE_SECRET"); err != nil {
		l.Fatal("the following error occurred:", zap.Error(err))
	}

	gitlabClient, err := NewClient()
	if err != nil {
		l.Fatal("initialising the gitlab client failed", zap.Error(err))
	}

	yamlConfig, err := readConfig()
	if err != nil {
		l.Fatal("reading the yamlConfig failed", zap.Error(err))
	}

	for range time.Tick(delay) {
		for index, repo := range yamlConfig.Repos {
			ctx := context.Background()

			context.WithValue(ctx, "token", repo.Name)
			if err := repo.CheckKeyRotationAndTokenAge(); err != nil {
				l.Warn(fmt.Sprintf("while processing %v at index %v, the following error occurred: %v", repo.Name, index, err))
				continue
			}
			if repo.GroupName != nil && repo.RepoName != nil {
				l.Warn(fmt.Sprintf("while processing %v at index %v, the following error occurred: %v", repo.Name, index, ErrGroupAndRepoDefined))
				continue
			}
			if repo.GroupName != nil {
				context.WithValue(ctx, "group", repo.GroupName)
				processGroupTokens(ctx, gitlabClient, &repo, index, yamlConfig)
				err := group.DeleteGroupTokens(gitlabClient, &repo, yamlConfig.Prefix)
				if err != nil {
					return
				}
			}
			if repo.RepoName != nil {
				context.WithValue(ctx, "repository", repo.RepoName)
				processProjectTokens(ctx, gitlabClient, &repo, index, yamlConfig)
				err := project.DeleteProjectTokens(gitlabClient, &repo, yamlConfig.Prefix)
				if err != nil {
					return
				}
			}
		}
	}
}
