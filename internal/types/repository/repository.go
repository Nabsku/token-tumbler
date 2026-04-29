package repository

import (
	"errors"
	"fmt"
	"github.com/nabsku/token-tumbler/internal/helper"
	"github.com/nabsku/token-tumbler/internal/logger"
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
	ErrMissingTokenExpiry           = errors.New("missing token expiry date")
	ErrInvalidRepositoryConfig      = errors.New("invalid repository configuration")
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
	if str == "" {
		return ErrMissingDuration
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
	}

	l.Error("Invalid repository prefix detected")
	return ErrInvalidPrefix
}

func (c *Config) Validate() error {
	if err := c.ValidatePrefix(); err != nil {
		return err
	}
	if len(c.Repos) == 0 {
		return fmt.Errorf("%w: repositories cannot be empty", ErrInvalidRepositoryConfig)
	}

	for index := range c.Repos {
		if err := c.Repos[index].Validate(); err != nil {
			return fmt.Errorf("repository at index %d: %w", index, err)
		}
	}
	if err := c.validateUniqueTokenTargets(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateUniqueTokenTargets() error {
	seen := make(map[string]int, len(c.Repos))
	for index := range c.Repos {
		identity := c.Repos[index].tokenIdentity(c.Prefix)
		firstIndex, exists := seen[identity]
		if exists {
			return fmt.Errorf("%w: duplicate token target at indexes %d and %d", ErrInvalidRepositoryConfig, firstIndex, index)
		}
		seen[identity] = index
	}
	return nil
}

func (c *Config) UsesVault() bool {
	for _, repo := range c.Repos {
		if strings.EqualFold(strings.TrimSpace(repo.SecretStore), "vault") {
			return true
		}
	}
	return false
}

func (r *Repository) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidRepositoryConfig)
	}
	if len(r.Permissions) == 0 {
		return fmt.Errorf("%w: permissions are required", ErrInvalidRepositoryConfig)
	}
	if r.RotationThreshold == nil {
		return fmt.Errorf("%w: rotationThreshold is required", ErrInvalidRepositoryConfig)
	}
	if r.GracePeriod == nil {
		return fmt.Errorf("%w: gracePeriod is required", ErrInvalidRepositoryConfig)
	}
	if r.Lifetime.ToDuration() == 0 {
		return fmt.Errorf("%w: lifetime is required", ErrInvalidRepositoryConfig)
	}
	if err := r.validateDurations(); err != nil {
		return err
	}
	if strings.TrimSpace(r.SecretStore) == "" {
		return fmt.Errorf("%w: secretStore is required; use \"none\" to disable persistence explicitly", ErrInvalidRepositoryConfig)
	}
	if err := r.validateTarget(); err != nil {
		return err
	}
	if err := r.validateSecretStore(); err != nil {
		return err
	}
	return r.CheckKeyRotationAndTokenAge()
}

func (r *Repository) validateDurations() error {
	if r.Lifetime.ToDuration() <= 0 {
		return fmt.Errorf("%w: lifetime must be greater than zero", ErrInvalidRepositoryConfig)
	}
	if r.RotationThreshold.ToDuration() <= 0 {
		return fmt.Errorf("%w: rotationThreshold must be greater than zero", ErrInvalidRepositoryConfig)
	}
	if r.GracePeriod.ToDuration() < 0 {
		return fmt.Errorf("%w: gracePeriod cannot be negative", ErrInvalidRepositoryConfig)
	}
	return nil
}

func (r *Repository) validateTarget() error {
	hasRepo := r.RepoName != nil && strings.TrimSpace(*r.RepoName) != ""
	hasGroup := r.GroupName != nil && strings.TrimSpace(*r.GroupName) != ""

	switch {
	case hasRepo && hasGroup:
		return fmt.Errorf("%w: define either repoName or groupName, not both", ErrInvalidRepositoryConfig)
	case !hasRepo && !hasGroup:
		return fmt.Errorf("%w: repoName or groupName is required", ErrInvalidRepositoryConfig)
	default:
		return nil
	}
}

func (r *Repository) validateSecretStore() error {
	switch strings.ToLower(strings.TrimSpace(r.SecretStore)) {
	case "none":
		return nil
	case "vault":
		if r.VaultPath == nil || strings.TrimSpace(*r.VaultPath) == "" {
			return fmt.Errorf("%w: vaultPath is required for vault secret store", ErrInvalidRepositoryConfig)
		}
		if r.VaultKey == nil || strings.TrimSpace(*r.VaultKey) == "" {
			return fmt.Errorf("%w: vaultKey is required for vault secret store", ErrInvalidRepositoryConfig)
		}
		if r.Mount == nil || strings.TrimSpace(*r.Mount) == "" {
			return fmt.Errorf("%w: vaultMount is required for vault secret store", ErrInvalidRepositoryConfig)
		}
		return nil
	default:
		return fmt.Errorf("%w: unsupported secret store %q", ErrInvalidRepositoryConfig, r.SecretStore)
	}
}

func (r *Repository) tokenIdentity(prefix string) string {
	targetType := "project"
	target := ""
	if r.GroupName != nil {
		targetType = "group"
		target = strings.TrimSpace(*r.GroupName)
	} else if r.RepoName != nil {
		target = strings.TrimSpace(*r.RepoName)
	}

	return strings.Join([]string{
		strings.TrimSpace(prefix),
		targetType,
		target,
		strings.TrimSpace(r.Name),
	}, "\x00")
}

func (r *Repository) ParseTokenName(prefix string, token string) (bool, error) {
	format := tokenNamePrefix(prefix, r.Name)
	if !strings.HasPrefix(token, format) {
		return false, fmt.Errorf("token %v does not adhere to format %v, skipping", token, format)
	}
	return true, nil
}

func (r *Repository) GetExpiryDate() (*time.Time, error) {
	expiryDate := time.Now().Add(r.Lifetime.ToDuration())

	return &expiryDate, nil
}

func (r *Repository) ShouldBeRenewed(token any) (bool, error) {
	l := logger.GetLogger()
	switch t := token.(type) {
	case *gitlab.ProjectAccessToken:
		l.Info("Checking project access token")
		return thresholdExceeded(r, t.ExpiresAt)

	case *gitlab.GroupAccessToken:
		l.Info("Checking group access token")
		return thresholdExceeded(r, t.ExpiresAt)
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
	if r.RotationThreshold == nil {
		return fmt.Errorf("%w: rotationThreshold is required", ErrInvalidRepositoryConfig)
	}
	if r.Lifetime.ToDuration() == r.RotationThreshold.ToDuration() {
		return ErrKeyAgeRotationSame
	}
	if r.Lifetime.ToDuration() < r.RotationThreshold.ToDuration() {
		return ErrTokenAgeLowerThanKeyRotation
	}

	return nil
}

func thresholdExceeded(r *Repository, expiresAt *gitlab.ISOTime) (bool, error) {
	if expiresAt == nil {
		return false, ErrMissingTokenExpiry
	}
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
