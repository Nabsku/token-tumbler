package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v3"
)

var (
	ErrTooManyGroupsInSearch        error = errors.New("There are too many groups in your query. Please narrow the group down by including the full path.")
	ErrGroupAndRepoDefined          error = errors.New("You cannot define both a Repository and Group name. Choose one or the other.")
	ErrKeyAgeRotationSame           error = errors.New("You cannot have the key rotation be the same as the maximum token age. This would result in many keys being created.")
	ErrTokenAgeLowerThanKeyRotation error = errors.New("You cannot set the maximum token age lower than key rotation threshold.")
	ErrTokenWithSameNameExists      error = errors.New("It is not allowed to set the same token name twice. This might occur when a token has already been renewed. In that case, ignore.")
)

const (
	delay       time.Duration = time.Duration(2) * time.Second
	errorString string        = "While processing %v at index %v, the following error occured: %w"
)

type Config struct {
	Repos       []Repository `yaml:"repos"`
	TokenPrefix TokenPrefix  `yaml:"tokenPrefix"`
}

type Repository struct {
	RepoName                   *RepoName   `yaml:"repoName,omitempty"`
	GroupName                  *GroupName  `yaml:"groupName,omitempty"`
	TokenName                  TokenName   `yaml:"tokenName"`
	Permissions                Permissions `yaml:"permissions"`
	KeyRotationThresholdInDays *int        `yaml:"keyRotationTresholdInDays,omitempty"`
	TokenLifetimeInDays        *int        `yaml:"tokenLifetimeInDays,omitempty"`
	RotateImmediately          *bool       `yaml:"rotateImmediately,omitempty"`
}

type (
	RepoName    string
	GroupName   string
	TokenName   string
	Permissions []string
	TokenPrefix string
)

func readConfig() (*Config, error) {
	buff, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	config := Config{}
	err = yaml.Unmarshal(buff, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (r *Repository) getFormattedRenewalDate(token *gitlab.GroupAccessToken) (gitlab.ISOTime, error) {
	time, err := time.Parse("2006-01-02", token.ExpiresAt.String())
	if err != nil {
		return gitlab.ISOTime{}, nil
	}
	date, err := gitlab.ParseISOTime(time.AddDate(0, 0, -*r.KeyRotationThresholdInDays).Format("2006-01-02"))
	log.Println(date)
	return date, err
}

func (r *Repository) getFormattedDate(token *gitlab.GroupAccessToken) (gitlab.ISOTime, error) {
	time, err := time.Parse("2006-01-02", token.ExpiresAt.String())
	if err != nil {
		return gitlab.ISOTime{}, nil
	}
	date, err := gitlab.ParseISOTime(time.Format("2006-01-02"))
	return date, err
}

func (r *Repository) getFormattedExpiryDate() (gitlab.ISOTime, error) {
	return gitlab.ParseISOTime(time.Now().AddDate(0, 0, *r.TokenLifetimeInDays).Format("2006-01-02"))
}

func (r *Repository) getNewFormattedTokenName(prefix *TokenPrefix) (TokenName, error) {
	// TODO: Some sort of error checking?
	return TokenName(*prefix) + r.TokenName + "-" + TokenName(time.Now().Format("2006-01-02")), nil
}

func (r *Repository) checkKeyRotationAndTokenAge() error {
	if *r.TokenLifetimeInDays == *r.KeyRotationThresholdInDays {
		return ErrKeyAgeRotationSame
	} else if *r.TokenLifetimeInDays < *r.KeyRotationThresholdInDays {
		return ErrTokenAgeLowerThanKeyRotation
	}

	return nil
}

func NewClient() (*gitlab.Client, error) {
	gitlabClient, err := gitlab.NewClient(os.Getenv("GITLAB_TOKEN"), gitlab.WithBaseURL(os.Getenv("GITLAB_URL")))
	if err != nil {
		return &gitlab.Client{}, err
	}

	return gitlabClient, nil
}

func gatherGroup(gitlabClient *gitlab.Client, entry *Repository) ([]*gitlab.Group, error) {
	opts := &gitlab.ListGroupsOptions{
		Search: gitlab.Ptr(string(*entry.GroupName)),
	}
	group, _, err := gitlabClient.Groups.ListGroups(opts)
	if err != nil {
		return []*gitlab.Group{}, err
	}

	if len(group) > 1 {
		return []*gitlab.Group{}, ErrTooManyGroupsInSearch
	}

	return group, nil
}

func gatherGroupTokenInfo(gitlabClient *gitlab.Client, groupID int) ([]*gitlab.GroupAccessToken, error) {
	groupTokens, _, err := gitlabClient.GroupAccessTokens.ListGroupAccessTokens(groupID, nil)
	if err != nil {
		return []*gitlab.GroupAccessToken{}, err
	}

	return groupTokens, nil
}

func checkTokenRenewal(entry *Repository, token *gitlab.GroupAccessToken) (bool, error) {
	date, err := entry.getFormattedRenewalDate(token)
	if err != nil {
		return false, err
	}

	if *token.ExpiresAt == date {
		return true, nil
	}

	if time.Time(*token.ExpiresAt).Before(time.Time(date)) {
		return true, nil
	}

	if token.CreatedAt.Before(token.CreatedAt.AddDate(0, 0, *entry.TokenLifetimeInDays)) {
		return true, nil
	}

	return false, nil
}

func parseTokenName(entry *Repository, token *gitlab.GroupAccessToken, prefix *TokenPrefix) (bool, error) {
	// if currentName, _ := entry.getNewFormattedTokenName(prefix); currentName == TokenName(token.Name) {
	// 	return false, ErrTokenWithSameNameExists
	// }
	format := TokenName(*prefix) + entry.TokenName + "-"
	if strings.Contains(token.Name, string(format)) {
		return true, nil // TODO: Return proper errors
	}

	return false, nil // TODO: Return proper errors
}

func formatTokenName(entry *Repository, prefix *TokenPrefix) (TokenName, error) {
	// TODO: Some sort of error checking?
	return TokenName(*prefix) + entry.TokenName + "-" + TokenName(time.Now().Format("2006-01-02")), nil
}

func createAccessTokenOptions(tokenName TokenName, scopes Permissions, expiry time.Time) *gitlab.CreateGroupAccessTokenOptions {
	return &gitlab.CreateGroupAccessTokenOptions{
		Name:      (*string)(&tokenName),
		Scopes:    (*[]string)(&scopes),
		ExpiresAt: (*gitlab.ISOTime)(&expiry),
	}
}

func createNewGroupToken(gitlabClient *gitlab.Client, groupID int, entry *Repository, prefix *TokenPrefix) (*gitlab.GroupAccessToken, error) {
	expireAtInDays := time.Time{}
	if entry.TokenLifetimeInDays == nil {
		expireAt := time.Now().AddDate(1, 0, 0)
		log.Printf("No expiration date set for %v, setting to %v\n", entry.TokenName, expireAt)
		expireAtInDays = expireAt
	} else {
		expireAtInDaysCheck, err := entry.getFormattedExpiryDate()
		fmt.Println(expireAtInDaysCheck)
		if err != nil {
			return nil, nil
		}
		expireAtInDays = time.Time(expireAtInDaysCheck)
	}

	tokenName, err := entry.getNewFormattedTokenName(prefix)
	if err != nil {
		return &gitlab.GroupAccessToken{}, err
	}

	opts := createAccessTokenOptions(tokenName, entry.Permissions, expireAtInDays)

	token, _, err := gitlabClient.GroupAccessTokens.CreateGroupAccessToken(groupID, opts)
	if err != nil {
		return &gitlab.GroupAccessToken{}, err
	}

	return token, nil
}

func renewGroupAccessToken(gitlabClient *gitlab.Client, groupID int, entry *Repository, prefix *TokenPrefix) (*gitlab.GroupAccessToken, error) {
	tokenName, _ := entry.getNewFormattedTokenName(prefix)
	expiryDate, _ := entry.getFormattedExpiryDate()
	opts := createAccessTokenOptions(tokenName, entry.Permissions, time.Time(expiryDate))

	token, _, err := gitlabClient.GroupAccessTokens.CreateGroupAccessToken(groupID, opts)
	if err != nil {
		return &gitlab.GroupAccessToken{}, err
	}

	return token, nil
}

func checkTokensForRenewal(tokens []*gitlab.GroupAccessToken, entry *Repository) (bool, error) {
	var newestValidToken *gitlab.GroupAccessToken
	var tokensToRenew []*gitlab.GroupAccessToken

	for _, token := range tokens {
		needsRenewal, err := checkTokenRenewal(entry, token)
		if err != nil {
			return false, err
		}

		if needsRenewal {
			tokensToRenew = append(tokensToRenew, token)
		}

		if newestValidToken == nil || token.CreatedAt.After(*newestValidToken.CreatedAt) {
			newestValidToken = token
		}
	}

	if len(tokensToRenew) == 1 {
		return true, nil
	}
	if len(tokensToRenew) != len(tokens) {
		log.Print("At least one active token found, no need to create another one")
		return false, nil
	}

	return false, nil

	// 	if needsRenewal {
	// 		if tokensToRenew == nil || time.Time(*token.ExpiresAt).Before(time.Time(*tokensToRenew.ExpiresAt)) {
	// 			tokensToRenew = token
	// 		}
	// 	} else {
	// 		if newestValidToken == nil || token.CreatedAt.After(*newestValidToken.CreatedAt) {
	// 			newestValidToken = token
	// 		}
	// 	}
	// }

	// If there's a valid token and it's at least two days old, we can renew the old one
	// if tokensToRenew != nil && newestValidToken != nil && isTokenAtLeastTwoDaysOld(newestValidToken) {
	// 	return true, nil
	// }
	//
	// // If there's no valid token, but we have a token that needs renewal, renew it
	// if newestValidToken == nil && tokensToRenew != nil {
	// 	return true, nil
	// }
	//
	// // In all other cases, don't create a new token
	// return false, nil
}

func gatherProjectAccessTokenInfo(entry *Config) (gitlab.ProjectAccessToken, error) {
	fmt.Println(entry)
	return gitlab.ProjectAccessToken{}, nil
}

func gatherPersonalAccessTokenInfo(entry *Config) (gitlab.PersonalAccessToken, error) {
	fmt.Println(entry)
	return gitlab.PersonalAccessToken{}, nil
}

func main() {
	if string, ok := os.LookupEnv("GITLAB_TOKEN"); !ok && len(string) <= 0 {
		log.Fatal("GITLAB_TOKEN env var not defined")
	}

	if string, ok := os.LookupEnv("GITLAB_URL"); !ok && len(string) <= 0 {
		log.Fatal("GITLAB_URL env var not defined")
	}

	gitlabClient, err := NewClient()
	if err != nil {
		log.Fatalf("Initialising the gitlab client failed with: %v", err)
	}

	config, err := readConfig()
	if err != nil {
		log.Fatalf("Reading the config failed with: %v", err)
	}

	for range time.Tick(delay) {
		for index, configEntry := range config.Repos {
			if err := configEntry.checkKeyRotationAndTokenAge(); err != nil {
				log.Println(fmt.Errorf(errorString, configEntry.TokenName, index, err))
				continue
			}
			if configEntry.GroupName != nil && configEntry.RepoName != nil {
				log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, ErrGroupAndRepoDefined))
				continue
			}
			if configEntry.GroupName != nil {
				info, err := gatherGroup(gitlabClient, &configEntry)
				if err != nil && err == ErrTooManyGroupsInSearch {
					log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, err))
					continue
				} else if err != nil {
					log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, err))
					continue
				}

				if len(info) < 1 {
					log.Printf("No group returned for %v, skipping\n", *configEntry.GroupName)
					continue
				}

				tokenInfo, err := gatherGroupTokenInfo(gitlabClient, info[0].ID)
				if err != nil {
					log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, err))
					continue
				}

				if len(tokenInfo) < 1 {
					log.Println("No token yet, we're free to create one as we please.")
					token, errTokenCreation := createNewGroupToken(gitlabClient, info[0].ID, &configEntry, &config.TokenPrefix)
					if errTokenCreation != nil {
						log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, errTokenCreation))
					}
					fmt.Println(token)
					break
				}

				tokenQueue := []*gitlab.GroupAccessToken{}
				for _, token := range tokenInfo {
					if parseOk, errTokenParse := parseTokenName(&configEntry, token, &config.TokenPrefix); parseOk {
						tokenQueue = append(tokenQueue, token)
					} else if errTokenParse != nil {
						log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, errTokenParse))
						break
					} else {
						log.Printf("Token %v found in Group %v does not match format, ignoring.\n", token.Name, *configEntry.GroupName)
						continue
					}
				}

				needsRenewal, err := checkTokensForRenewal(tokenQueue, &configEntry)
				if err != nil {
					log.Println(fmt.Errorf(errorString, *configEntry.GroupName, index, err))
					continue
				}

				if needsRenewal {
					log.Printf("Token for %v in Group %v is ready to be renewed.\n", configEntry.TokenName, *configEntry.GroupName)
					token, errRenewal := renewGroupAccessToken(gitlabClient, info[0].ID, &configEntry, &config.TokenPrefix)
					if errRenewal != nil {
						log.Println(fmt.Errorf(errorString, configEntry.TokenName, index, errRenewal))
					}
					fmt.Println(token)
				} else {
					log.Printf("No tokens for %v in Group %v need renewal at this time.\n", configEntry.TokenName, *configEntry.GroupName)
				}

				// if renewalOk, errTokenRenewalInitCheck := checkTokenRenewal(&configEntry, token); renewalOk {
				// 	log.Printf("Token %v in Group %v is ready to be renewed.\n", &configEntry.TokenName, *configEntry.GroupName)
				// 	token, errRenewal := renewGroupAccessToken(gitlabClient, info[0].ID, &configEntry, &config.TokenPrefix)
				// 	if errRenewal != nil {
				// 		log.Println(fmt.Errorf(errorString, &configEntry.TokenName, index, errRenewal))
				// 	}
				// 	fmt.Println(token)
				// 	break
				// } else if errTokenRenewalInitCheck != nil {
				// 	log.Println(fmt.Errorf(errorString, &configEntry.TokenName, index, errTokenRenewalInitCheck))
				// 	break
				// } else {
				// 	log.Printf("Token %v in Group %v is not ready to be renewed yet.\n", configEntry.TokenName, *configEntry.GroupName)
				// 	break
				// }
			}
			// fmt.Println(tokenQueue)
		}
	}
}

// }
