package config

import (
	"cmp"
	"os"
)

type Config struct {
	GitlabUrl   string
	GitlabToken string
}

func NewConfig() *Config {
	host := cmp.Or(os.Getenv("GITLAB_URL"), "localhost")
	token := cmp.Or(os.Getenv("GITLAB_TOKEN"), "faketoken")
	return &Config{
		GitlabUrl:   host,
		GitlabToken: token,
	}
}
