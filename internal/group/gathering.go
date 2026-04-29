package group

import (
	"errors"
	"net/http"

	"github.com/nabsku/token-chaser/internal/types/repository"

	"gitlab.com/gitlab-org/api/client-go"
)

var ErrTooManyGroupsInSearch = errors.New("there are too many groups in your query. please narrow the group down by including the full path")
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

func GatherGroupTokenInfo(gitlabClient *gitlab.Client, groupID int) ([]*gitlab.GroupAccessToken, error) {
	groupTokens, _, err := gitlabClient.GroupAccessTokens.ListGroupAccessTokens(groupID, nil)
	if err != nil {
		return nil, err
	}

	return groupTokens, nil
}

func GatherGroupTokenInfoByPrefix(gitlabClient *gitlab.Client, groupID int, prefix string, entry repository.Repository) ([]*gitlab.GroupAccessToken, error) {
	groupTokens, _, err := gitlabClient.GroupAccessTokens.ListGroupAccessTokens(groupID, nil)
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
