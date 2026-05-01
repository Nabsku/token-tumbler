package leaderelection

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnabledEnvVar       = "TOKEN_TUMBLER_LEADER_ELECTION_ENABLED"
	NamespaceEnvVar     = "TOKEN_TUMBLER_LEADER_ELECTION_NAMESPACE"
	LeaseNameEnvVar     = "TOKEN_TUMBLER_LEADER_ELECTION_LEASE_NAME"
	IdentityEnvVar      = "TOKEN_TUMBLER_LEADER_ELECTION_IDENTITY"
	LeaseDurationEnvVar = "TOKEN_TUMBLER_LEADER_ELECTION_LEASE_DURATION"
	RenewDeadlineEnvVar = "TOKEN_TUMBLER_LEADER_ELECTION_RENEW_DEADLINE"
	RetryPeriodEnvVar   = "TOKEN_TUMBLER_LEADER_ELECTION_RETRY_PERIOD"

	defaultLeaseName     = "token-tumbler"
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
)

type Config struct {
	Enabled       bool
	Namespace     string
	LeaseName     string
	Identity      string
	LeaseDuration time.Duration
	RenewDeadline time.Duration
	RetryPeriod   time.Duration
}

func ConfigFromEnv() (Config, error) {
	enabled, err := boolFromEnv(EnabledEnvVar)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Enabled:       enabled,
		Namespace:     strings.TrimSpace(os.Getenv(NamespaceEnvVar)),
		LeaseName:     strings.TrimSpace(os.Getenv(LeaseNameEnvVar)),
		Identity:      strings.TrimSpace(os.Getenv(IdentityEnvVar)),
		LeaseDuration: defaultLeaseDuration,
		RenewDeadline: defaultRenewDeadline,
		RetryPeriod:   defaultRetryPeriod,
	}
	if cfg.LeaseName == "" {
		cfg.LeaseName = defaultLeaseName
	}
	if cfg.Identity == "" {
		cfg.Identity, _ = os.Hostname()
	}

	if cfg.LeaseDuration, err = durationFromEnv(LeaseDurationEnvVar, cfg.LeaseDuration); err != nil {
		return Config{}, err
	}
	if cfg.RenewDeadline, err = durationFromEnv(RenewDeadlineEnvVar, cfg.RenewDeadline); err != nil {
		return Config{}, err
	}
	if cfg.RetryPeriod, err = durationFromEnv(RetryPeriodEnvVar, cfg.RetryPeriod); err != nil {
		return Config{}, err
	}

	if cfg.Enabled {
		if cfg.Namespace == "" {
			return Config{}, fmt.Errorf("%s must be set when leader election is enabled", NamespaceEnvVar)
		}
		if cfg.Identity == "" {
			return Config{}, fmt.Errorf("%s or hostname must be set when leader election is enabled", IdentityEnvVar)
		}
		if cfg.LeaseDuration <= cfg.RenewDeadline {
			return Config{}, fmt.Errorf("%s must be greater than %s", LeaseDurationEnvVar, RenewDeadlineEnvVar)
		}
		if cfg.RenewDeadline <= cfg.RetryPeriod {
			return Config{}, fmt.Errorf("%s must be greater than %s", RenewDeadlineEnvVar, RetryPeriodEnvVar)
		}
	}

	return cfg, nil
}

func boolFromEnv(name string) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return false, nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", name, err)
	}
	return enabled, nil
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("invalid %s: must be greater than zero", name)
	}
	return duration, nil
}
