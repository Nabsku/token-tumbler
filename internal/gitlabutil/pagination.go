package gitlabutil

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/api/client-go"
)

type PageFunc[T any] func() ([]T, *gitlab.Response, error)
type SetPageFunc func(page int64)

var (
	ErrPaginationLoop        = errors.New("pagination loop detected")
	ErrPaginationInvalidPage = errors.New("pagination returned non-advancing page")
	ErrPaginationPageLimit   = errors.New("pagination exceeded maximum pages")
)

const maxPagesPerList = int64(1000)

func CollectPages[T any](ctx context.Context, fetch PageFunc[T], setPage SetPageFunc) ([]T, error) {
	if setPage == nil {
		return nil, errors.New("setPage callback is required")
	}

	var all []T
	var pagesFetched int64
	seenPages := map[int64]struct{}{}
	currentPage := int64(0)

	for {
		if pagesFetched >= maxPagesPerList {
			return nil, fmt.Errorf("%w: %d", ErrPaginationPageLimit, maxPagesPerList)
		}
		pagesFetched++
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
		if response.NextPage <= currentPage {
			return nil, fmt.Errorf("%w: next=%d current=%d", ErrPaginationInvalidPage, response.NextPage, currentPage)
		}
		if _, ok := seenPages[response.NextPage]; ok {
			return nil, fmt.Errorf("%w: %d", ErrPaginationLoop, response.NextPage)
		}
		seenPages[response.NextPage] = struct{}{}
		currentPage = response.NextPage
		setPage(response.NextPage)
	}
	return all, nil
}
