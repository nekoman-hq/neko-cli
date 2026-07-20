package releaseit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
)

type Config struct {
	Github *GithubRelease `json:"github"`
	Git    *GitConfig     `json:"git,omitempty"`
	Hooks  *HooksConfig   `json:"hooks,omitempty"`
	Schema string         `json:"$schema"`
}

type GithubRelease struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Release     bool   `json:"release"`
}

type GitConfig struct {
	Changelog                                 string `json:"changelog,omitempty"`
	CommitMessage                             string `json:"commitMessage,omitempty"`
	Commit, Tag, Push, RequireCleanWorkingDir bool
}

type HooksConfig struct {
	AfterBump string `json:"after:bump,omitempty"`
}

func LoadConfig() (*Config, error) {
	return LoadConfigAt("")
}

func LoadConfigAt(repositoryRoot string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(repositoryRoot, releasetool.ReleaseItConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	return SaveConfigAt("", config)
}

func SaveConfigAt(repositoryRoot string, config *Config) (err error) {
	file, err := os.Create(filepath.Join(repositoryRoot, releasetool.ReleaseItConfigFile))
	if err != nil {
		return fmt.Errorf("create .release-it.json: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close file: %w", closeErr)
		}
	}()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(config); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}

func InitDefaultConfig(projectName string) (*Config, error) {
	return &Config{
		Schema: "https://unpkg.com/release-it/schema/release-it.json",
		Github: &GithubRelease{
			Release:     true,
			ReleaseName: fmt.Sprintf("%s@${version}", projectName),
		},
		Git: &GitConfig{
			Commit:                 true,
			Tag:                    true,
			Push:                   true,
			RequireCleanWorkingDir: true,
			Changelog:              "npx auto-changelog --stdout --commit-limit false -u --template https://raw.githubusercontent.com/release-it/release-it/main/templates/changelog-compact.hbs",
			CommitMessage:          "chore(release): ${version}",
		},
		Hooks: &HooksConfig{
			AfterBump: "npx auto-changelog -p",
		},
	}, nil
}
