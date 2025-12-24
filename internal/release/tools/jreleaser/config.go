package jreleaser

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      24.12.2025
*/

import (
	"os"

	"github.com/nekoman-hq/neko-cli/internal/errors"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Project Project `yaml:"project"`
	Release Release `yaml:"release"`
}

type Project struct {
	Name          string           `yaml:"name"`
	Version       string           `yaml:"version"`
	Description   string           `yaml:"description,omitempty"`
	LongDesc      string           `yaml:"longDescription,omitempty"`
	Authors       []string         `yaml:"authors"`
	License       string           `yaml:"license"`
	Links         ProjectLinks     `yaml:"links"`
	Languages     ProjectLanguages `yaml:"languages"`
	InceptionYear string           `yaml:"inceptionYear"`
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
	Overwrite   bool      `yaml:"overwrite"`
	Owner       string    `yaml:"owner"`
	Name        string    `yaml:"name"`
	TagName     string    `yaml:"tagName"`
	ReleaseName string    `yaml:"releaseName"`
	Changelog   Changelog `yaml:"changelog"`
}

type Changelog struct {
	Enabled          bool            `yaml:"enabled"`
	Sort             string          `yaml:"sort"`
	SkipMergeCommits bool            `yaml:"skipMergeCommits"`
	Formatted        string          `yaml:"formatted"`
	Preset           string          `yaml:"preset"`
	Contributors     Contributors    `yaml:"contributors"`
	Append           ChangelogAppend `yaml:"append"`
	IncludeLabels    []string        `yaml:"includeLabels"`
	Labelers         []Labeler       `yaml:"labelers"`
	Categories       []Category      `yaml:"categories"`
}

type Contributors struct {
	Enabled bool   `yaml:"enabled"`
	Format  string `yaml:"format,omitempty"`
}

type ChangelogAppend struct {
	Enabled bool   `yaml:"enabled"`
	Title   string `yaml:"title"`
	Target  string `yaml:"target"`
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

func SaveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		errors.Fatal(
			"Configuration serialization failed",
			"Could not marshal jreleaser.yml: "+err.Error(),
			errors.ErrConfigMarshal,
		)
		return err
	}

	if err := os.WriteFile("jreleaser.yml", data, 0644); err != nil {
		errors.Fatal(
			"Configuration write failed",
			"Could not write jreleaser.yml: "+err.Error(),
			errors.ErrConfigWrite,
		)
		return err
	}
	return nil
}
