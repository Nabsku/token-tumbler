package gitlabutil

import (
	"context"

	"gitlab.com/gitlab-org/api/client-go"
)

type PageFunc[T any] func() ([]T, *gitlab.Response, error)
type SetPageFunc func(page int64)

func CollectPages[T any](ctx context.Context, fetch PageFunc[T], setPage SetPageFunc) ([]T, error) {
	var all []T
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pageItems, response, err := fetch()
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, err
		}
		all = append(all, pageItems...)
		if response == nil || response.NextPage == 0 {
			break
		}
		setPage(response.NextPage)
	}
	return all, nil
}
