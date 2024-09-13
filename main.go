package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/xanzy/go-gitlab"
	"gopkg.in/yaml.v3"
)

const delay time.Duration = time.Duration(2) * time.Second

type Config struct {
	Repos []Repository `yaml:"repos"`
}

type Repository struct {
	RepoName       *RepoName   `yaml:"repoName,omitempty"`
	GroupName      *GroupName  `yaml:"groupName,omitempty"`
	TokenName      TokenName   `yaml:"tokenName"`
	Permissions    Permissions `yaml:"permissions"`
	UpdateWhenDays *int        `yaml:"updateWhenDays,omitempty"`
}

type (
	RepoName    string
	GroupName   string
	TokenName   string
	Permissions []string
)

func readConfig() (*Config, error) {
	buff, err := os.ReadFile("config.yaml")
	if err != nil {
		return nil, err
	}

	config := Config{}
	errUnmarshal := yaml.Unmarshal(buff, &config)

	if errUnmarshal != nil {
		return nil, errUnmarshal
	}

	return &config, nil
}

func NewClient() (*gitlab.Client, error) {
	gitlabClient, err := gitlab.NewClient(os.Getenv("GITLAB_TOKEN"), gitlab.WithBaseURL(os.Getenv("GITLAB_URL")))
	if err != nil {
		return &gitlab.Client{}, err
	}

	return gitlabClient, nil
}

func gatherGroupTokenInfo(gitlabClient *gitlab.Client, entry *Repository) (gitlab.GroupAccessToken, error) {
	fmt.Println(entry)
	return gitlab.GroupAccessToken{}, nil
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

	// fmt.Print(gitlabClient)

	config, err := readConfig()
	if err != nil {
		log.Fatalf("Reading the config failed with: %v", err)
	}

	for range time.Tick(delay) {
		for _, v := range config.Repos {
			if v.GroupName != nil && v.RepoName != nil {
				log.Fatalf("You cannot define both a Repository and Group name. Choose one or the other. Occured while processing: Token %v | Repo %v | Group %v", v.TokenName, *v.RepoName, *v.GroupName)
			}
			if v.GroupName != nil {
				gatherGroupTokenInfo(gitlabClient, &v)
			}
		}
	}
}
