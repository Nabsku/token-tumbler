package repository

import (
	"errors"
	"fmt"
	"github.com/nabsku/token-chaser/internal/helper"
	"github.com/nabsku/token-chaser/internal/logger"
	"strconv"
	"strings"
	"time"

	"gitlab.com/gitlab-org/api/client-go"
)

var (
	ErrKeyAgeRotationSame           = errors.New("you cannot have the key rotation be the same as the maximum token age. this would result in many keys being created")
	ErrTokenAgeLowerThanKeyRotation = errors.New("you cannot set the maximum token age lower than key rotation threshold")
	ErrInvalidPrefix                = errors.New("invalid prefix. only use alphanumeric characters, hyphens, or underscores")
	ErrInvalidTokenType             = errors.New("invalid token type")
	ErrMissingDuration              = errors.New("missing duration suffix, please use s, m, h, d, w or M")
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	l := logger.GetLogger()
	var str string
	if err := unmarshal(&str); err != nil {
		return err
	}

	unit := str[len(str)-1]
	l.Debug(fmt.Sprintf("Unit: %c", unit))
	value, err := strconv.Atoi(str[:len(str)-1])
	l.Debug(fmt.Sprintf("Value: %d", value))

	if errors.Is(err, strconv.ErrSyntax) {
		l.Debug(fmt.Sprintf("Invalid token value: %s", str))
		return ErrMissingDuration
	} else if err != nil {
		return err
	}

	switch unit {
	case 's':
		d.Duration = time.Duration(value) * time.Second
	case 'm':
		d.Duration = time.Duration(value) * time.Minute
	case 'h':
		d.Duration = time.Duration(value) * time.Hour
	case 'd':
		d.Duration = time.Duration(value) * 24 * time.Hour
	case 'w':
		d.Duration = time.Duration(value) * 7 * 24 * time.Hour
	case 'M':
		d.Duration = time.Duration(value) * 30 * 24 * time.Hour
	default:
		return fmt.Errorf("invalid duration unit: %c", unit)
	}

	return nil
}

func (d *Duration) ToDuration() time.Duration {
	return d.Duration
}

type (
	Config struct {
		Repos  []Repository `yaml:"repositories"`
		Prefix string       `yaml:"prefix"`
	}
	Repository struct {
		RepoName          *string   `yaml:"repoName,omitempty"`
		GroupName         *string   `yaml:"groupName,omitempty"`
		Name              string    `yaml:"name"`
		Permissions       []string  `yaml:"permissions"`
		RotationThreshold *Duration `yaml:"rotationThreshold"`
		GracePeriod       *Duration `yaml:"gracePeriod"`
		Lifetime          Duration  `yaml:"lifetime"`
		SecretStore       string    `yaml:"secretStore,omitempty"`
		VaultPath         *string   `yaml:"vaultPath,omitempty"`
		VaultKey          *string   `yaml:"vaultKey,omitempty"`
		Mount             *string   `yaml:"vaultMount,omitempty"`
	}
)

func (c *Config) ValidatePrefix() error {
	l := logger.GetLogger()

	if helper.IsLetter(c.Prefix) {
		l.Info("Validating repository prefix detected")
		return nil
	} else {
		l.Error("Invalid repository prefix detected")
		return ErrInvalidPrefix
	}
}

func (r *Repository) GetRenewalDate(expiryDate *gitlab.ISOTime) (*gitlab.ISOTime, error) {
	parsedTime, err := parseISOTime(expiryDate.String())
	if err != nil {
		return nil, err
	}

	date, err := gitlab.ParseISOTime(parsedTime.Add(-r.RotationThreshold.ToDuration()).Format(time.DateOnly))
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func (r *Repository) ParseTokenName(prefix string, token string) (bool, error) {
	format := tokenNamePrefix(prefix, r.Name)
	if !strings.HasPrefix(token, format) {
		return false, fmt.Errorf("token %v does not adhere to format %v, skipping", token, format)
	}
	return true, nil
}

//func (r *Repository) GetDate(expiryDate *gitlab.ISOTime) (*gitlab.ISOTime, error) {
//	parsedTime, err := time.Parse(time.RFC3339, expiryDate.String())
//	if err != nil {
//		return nil, err
//	}
//	date, err := gitlab.ParseISOTime(parsedTime.Format(time.RFC3339))
//	return &date, err
//}

func (r *Repository) GetExpiryDate() (*time.Time, error) {
	expiryDate := time.Now().Add(r.Lifetime.ToDuration())

	return &expiryDate, nil
}

func (r *Repository) ShouldBeRenewed(token any) (bool, error) {
	l := logger.GetLogger()
	switch t := token.(type) {
	case *gitlab.ProjectAccessToken:
		l.Info("Checking project access token")
		threshold, err := thresholdExceeded(r, t.ExpiresAt)
		if err != nil {
			return false, err
		}

		if threshold {
			return true, nil
		}
		return false, nil

	case *gitlab.GroupAccessToken:
		l.Info("Checking group access token")
		threshold, err := thresholdExceeded(r, t.ExpiresAt)
		if err != nil {
			return false, err
		}

		if threshold {
			return true, nil
		}
		return false, nil
	case *gitlab.PersonalAccessToken:
		l.Warn("Personal access token is not supported yet")
		return false, fmt.Errorf("personal access token is not supported yet, %w", ErrInvalidTokenType)
	default:
		return false, ErrInvalidTokenType
	}
}

func (r *Repository) NewTokenName(prefix string) (string, error) {
	if prefix == "" {
		return "", ErrInvalidPrefix
	}
	if r.Name == "" {
		return "", fmt.Errorf("repository token name cannot be empty")
	}

	return tokenNamePrefix(prefix, r.Name) + time.Now().Format(time.RFC3339), nil
}

func (r *Repository) CheckKeyRotationAndTokenAge() error {
	if r.Lifetime.ToDuration() == r.RotationThreshold.ToDuration() {
		return ErrKeyAgeRotationSame
	} else if r.Lifetime.ToDuration() < r.RotationThreshold.ToDuration() {
		return ErrTokenAgeLowerThanKeyRotation
	}

	return nil
}

func thresholdExceeded(r *Repository, expiresAt *gitlab.ISOTime) (bool, error) {
	expiresAtTime, err := parseISOTime(expiresAt.String())
	if err != nil {
		return false, err
	}

	renewalThreshold := time.Now().Add(r.RotationThreshold.ToDuration())
	return !expiresAtTime.After(renewalThreshold), nil
}

func parseISOTime(value string) (time.Time, error) {
	parsedTime, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsedTime, nil
	}

	dateOnlyTime, dateOnlyErr := time.Parse(time.DateOnly, value)
	if dateOnlyErr == nil {
		return dateOnlyTime, nil
	}

	return time.Time{}, err
}

func tokenNamePrefix(prefix, name string) string {
	return strings.TrimSuffix(prefix, "-") + "-" + name + "-"
}

//func (r *Repository) GetSecretStore() string {
//	if r.SecretStore != "" {
//		return r.SecretStore
//	}
//	return ""
//}
