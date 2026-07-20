package jreleaser

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Project Project `yaml:"project"`
	Release Release `yaml:"release"`
}

type Project struct {
	Authors   *[]string        `yaml:"authors,omitempty"`
	Languages ProjectLanguages `yaml:"languages"`
	Links     ProjectLinks     `yaml:"links"`

	Name          string `yaml:"name"`
	Version       string `yaml:"version"`
	License       string `yaml:"license"`
	InceptionYear string `yaml:"inceptionYear"`

	Description string `yaml:"description,omitempty"`
	LongDesc    string `yaml:"longDescription,omitempty"`
}

type ProjectLinks struct {
	Homepage string `yaml:"homepage"`
}

type ProjectLanguages struct {
	Java JavaLanguage `yaml:"java"`
}

type JavaLanguage struct {
	GroupID string `yaml:"groupId"`
	Version string `yaml:"version"`
}

type Release struct {
	Github GithubRelease `yaml:"github"`
}

type GithubRelease struct {
	Owner       string    `yaml:"owner"`
	Name        string    `yaml:"name"`
	TagName     string    `yaml:"tagName"`
	ReleaseName string    `yaml:"releaseName"`
	Changelog   Changelog `yaml:"changelog"`
	Overwrite   bool      `yaml:"overwrite"`
}

type Changelog struct {
	IncludeLabels *[]string        `yaml:"includeLabels,omitempty"`
	Labelers      *[]Labeler       `yaml:"labelers,omitempty"`
	Categories    *[]Category      `yaml:"categories,omitempty"`
	Contributors  *Contributors    `yaml:"contributors,omitempty"`
	Append        *ChangelogAppend `yaml:"append,omitempty"`

	Sort      string `yaml:"sort"`
	Formatted string `yaml:"formatted"`
	Preset    string `yaml:"preset"`

	Enabled          bool `yaml:"enabled"`
	SkipMergeCommits bool `yaml:"skipMergeCommits"`
}

type Contributors struct {
	Format  string `yaml:"format,omitempty"`
	Enabled bool   `yaml:"enabled"`
}

type ChangelogAppend struct {
	Title   string `yaml:"title"`
	Target  string `yaml:"target"`
	Enabled bool   `yaml:"enabled"`
}

type Labeler struct {
	Label string `yaml:"label"`
	Title string `yaml:"title"`
	Order int    `yaml:"order"`
}

type Category struct {
	Title  string   `yaml:"title"`
	Key    string   `yaml:"key"`
	Labels []string `yaml:"labels"`
	Order  int      `yaml:"order"`
}

func LoadConfig() (*Config, error) {
	return LoadConfigAt("")
}

func LoadConfigAt(repositoryRoot string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(repositoryRoot, releasetool.JReleaserConfigFile))
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func SaveConfig(config *Config) error {
	return SaveConfigAt("", config)
}

func SaveConfigAt(repositoryRoot string, config *Config) (err error) {
	file, err := os.Create(filepath.Join(repositoryRoot, releasetool.JReleaserConfigFile))
	if err != nil {
		return fmt.Errorf("create jreleaser.yml: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close file: %w", closeErr)
		}
	}()

	encoder := yaml.NewEncoder(file)
	defer func() {
		if closeErr := encoder.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close encoder: %w", closeErr)
		}
	}()

	encoder.SetIndent(2)
	if err = encoder.Encode(config); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}
