package group

import (
	"errors"
	"net/http"

	"github.com/nabsku/token-tumbler/internal/gitlabutil"
	"github.com/nabsku/token-tumbler/internal/types/repository"

	"gitlab.com/gitlab-org/api/client-go"
)

var ErrNoGroupsInSearch = errors.New("no groups found in your query")

func GatherGroup(gitlabClient *gitlab.Client, entry *repository.Repository) (*gitlab.Group, error) {
	group, response, err := gitlabClient.Groups.GetGroup(*entry.GroupName, nil)
	if err != nil {
		if response != nil && response.StatusCode == http.StatusNotFound {
			return nil, ErrNoGroupsInSearch
		}
		return nil, err
	}
	if group == nil {
		return nil, ErrNoGroupsInSearch
	}

	return group, nil
}

func GatherGroupTokenInfo(gitlabClient *gitlab.Client, groupID int64) ([]*gitlab.GroupAccessToken, error) {
	options := &gitlab.ListGroupAccessTokensOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	return gitlabutil.CollectPages(
		func() ([]*gitlab.GroupAccessToken, *gitlab.Response, error) {
			return gitlabClient.GroupAccessTokens.ListGroupAccessTokens(groupID, options)
		},
		func(page int64) {
			options.Page = page
		},
	)
}

func GatherGroupTokenInfoByPrefix(gitlabClient *gitlab.Client, groupID int64, prefix string, entry repository.Repository) ([]*gitlab.GroupAccessToken, error) {
	groupTokens, err := GatherGroupTokenInfo(gitlabClient, groupID)
	if err != nil {
		return nil, err
	}

	var prefixedTokens []*gitlab.GroupAccessToken

	for _, token := range groupTokens {
		if token.Revoked || !token.Active {
			continue
		}
		if ok, _ := entry.ParseTokenName(prefix, token.Name); ok {
			prefixedTokens = append(prefixedTokens, token)
		}
	}

	return prefixedTokens, nil
}
