package project

import (
	"context"
	"errors"
	"net/http"

	"github.com/nabsku/token-tumbler/internal/gitlabutil"
	"github.com/nabsku/token-tumbler/internal/types/repository"

	"gitlab.com/gitlab-org/api/client-go"
)

var ErrNoProjectsInSearch = errors.New("no projects found in your query")

func GatherProject(ctx context.Context, gitlabClient *gitlab.Client, entry *repository.Repository) (*gitlab.Project, error) {
	project, response, err := gitlabClient.Projects.GetProject(*entry.RepoName, nil, gitlab.WithContext(ctx))
	if err != nil {
		if response != nil && response.StatusCode == http.StatusNotFound {
			return nil, ErrNoProjectsInSearch
		}
		return nil, err
	}
	if project == nil {
		return nil, ErrNoProjectsInSearch
	}

	return project, nil
}

func GatherProjectTokenInfo(ctx context.Context, gitlabClient *gitlab.Client, projectID int64) ([]*gitlab.ProjectAccessToken, error) {
	options := &gitlab.ListProjectAccessTokensOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	return gitlabutil.CollectPages(
		ctx,
		func() ([]*gitlab.ProjectAccessToken, *gitlab.Response, error) {
			return gitlabClient.ProjectAccessTokens.ListProjectAccessTokens(projectID, options, gitlab.WithContext(ctx))
		},
		func(page int64) {
			options.Page = page
		},
	)
}
