package project

import (
	"errors"
	"github.com/nabsku/token-chaser/internal/types/repository"

	"gitlab.com/gitlab-org/api/client-go"
)

var ErrTooManyProjectsInSearch = errors.New("there are too many projects found in your query. please narrow the search by including the full path")
var ErrNoProjectsInSearch = errors.New("no projects found in your query")

func GatherProject(gitlabClient *gitlab.Client, entry *repository.Repository) (*gitlab.Project, error) {
	opts := &gitlab.ListProjectsOptions{
		Search: gitlab.Ptr(*entry.RepoName),
	}
	project, _, err := gitlabClient.Projects.ListProjects(opts)
	if err != nil {
		return nil, err
	}

	if len(project) > 1 {
		return nil, ErrTooManyProjectsInSearch
	}
	if len(project) == 0 {
		return nil, ErrNoProjectsInSearch
	}

	return project[0], nil
}

func GatherProjectTokenInfo(gitlabClient *gitlab.Client, projectID int) ([]*gitlab.ProjectAccessToken, error) {
	projectTokens, _, err := gitlabClient.ProjectAccessTokens.ListProjectAccessTokens(projectID, nil)
	if err != nil {
		return nil, err
	}

	return projectTokens, nil
}
