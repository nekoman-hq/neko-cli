package jreleaser

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      24.12.2025
*/

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Project Project `yaml:"project"`
	Release Release `yaml:"release"`
}

type Project struct {
	Authors   *[]string        `yaml:"authors"`
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
	IncludeLabels *[]string
	Labelers      *[]Labeler
	Categories    *[]Category
	Contributors  *Contributors
	Append        *ChangelogAppend

	Sort      string
	Formatted string
	Preset    string

	Enabled, SkipMergeCommits bool
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
	data, err := os.ReadFile("jreleaser.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

func SaveConfig(cfg *Config) (err error) {
	file, err := os.Create("jreleaser.yml")
	if err != nil {
		return fmt.Errorf("create jreleaser.yml: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close file: %w", cerr)
		}
	}()

	encoder := yaml.NewEncoder(file)
	defer func() {
		if cerr := encoder.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close encoder: %w", cerr)
		}
	}()

	encoder.SetIndent(2)

	if err = encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	return nil
}
