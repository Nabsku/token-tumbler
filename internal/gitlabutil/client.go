package gitlabutil

import (
	"net/http"
	"time"

	"github.com/nabsku/token-tumbler/internal/config"
	"gitlab.com/gitlab-org/api/client-go"
)

func NewClient() (*gitlab.Client, error) {
	newConfig := config.NewConfig()
	gitlabClient, err := gitlab.NewClient(
		newConfig.GitlabToken,
		gitlab.WithBaseURL(newConfig.GitlabUrl),
		gitlab.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
	)
	if err != nil {
		return nil, err
	}

	return gitlabClient, nil
}
