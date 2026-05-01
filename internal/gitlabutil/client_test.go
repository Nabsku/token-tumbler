package gitlabutil

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	t.Setenv("GITLAB_URL", "https://gitlab.example.com")
	t.Setenv("GITLAB_TOKEN", "token")

	client, err := NewClient()

	require.NoError(t, err)
	require.NotNil(t, client)
}
