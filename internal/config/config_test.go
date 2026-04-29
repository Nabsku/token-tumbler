package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetConfig_ShouldReturnConfigWithDefaultValues(t *testing.T) {
	t.Setenv("GITLAB_URL", "")
	t.Setenv("GITLAB_TOKEN", "")

	config := NewConfig()

	assert.Equal(t, "localhost", config.GitlabUrl)
	assert.Equal(t, "faketoken", config.GitlabToken)
}

func TestGetConfig_ShouldReturnConfigWithEnvValues(t *testing.T) {
	t.Setenv("GITLAB_URL", "https://gitlab.example.com")
	t.Setenv("GITLAB_TOKEN", "real-token")

	config := NewConfig()

	assert.Equal(t, "https://gitlab.example.com", config.GitlabUrl)
	assert.Equal(t, "real-token", config.GitlabToken)
}
