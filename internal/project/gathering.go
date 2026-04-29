package project

import (
	"errors"
	"net/http"

	"github.com/nabsku/token-tumbler/internal/types/repository"

	"gitlab.com/gitlab-org/api/client-go"
)

var ErrNoProjectsInSearch = errors.New("no projects found in your query")

func GatherProject(gitlabClient *gitlab.Client, entry *repository.Repository) (*gitlab.Project, error) {
	project, response, err := gitlabClient.Projects.GetProject(*entry.RepoName, nil)
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

func GatherProjectTokenInfo(gitlabClient *gitlab.Client, projectID int) ([]*gitlab.ProjectAccessToken, error) {
	options := &gitlab.ListProjectAccessTokensOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	var projectTokens []*gitlab.ProjectAccessToken

	for {
		pageTokens, response, err := gitlabClient.ProjectAccessTokens.ListProjectAccessTokens(projectID, options)
		if err != nil {
			return nil, err
		}
		projectTokens = append(projectTokens, pageTokens...)
		if response == nil || response.NextPage == 0 {
			break
		}
		options.Page = response.NextPage
	}

	return projectTokens, nil
}
