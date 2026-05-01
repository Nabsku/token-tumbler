package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/nabsku/token-tumbler/internal/types/repository"
)

const (
	defaultPollInterval = 5 * time.Minute
	pollIntervalEnvVar  = "TOKEN_TUMBLER_INTERVAL"
)

func ReadRepositoryConfig(filename string) (*repository.Config, error) {
	buff, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	repoConfig := repository.Config{}
	err = yaml.Unmarshal(buff, &repoConfig)
	if err != nil {
		return nil, err
	}
	if err := repoConfig.Validate(); err != nil {
		return nil, err
	}

	return &repoConfig, nil
}

func PollIntervalFromEnv() (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(pollIntervalEnvVar))
	if value == "" {
		return defaultPollInterval, nil
	}
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", pollIntervalEnvVar, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("invalid %s: must be greater than zero", pollIntervalEnvVar)
	}
	return interval, nil
}
