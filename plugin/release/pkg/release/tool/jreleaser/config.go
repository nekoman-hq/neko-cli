package jreleaser

import jreleaserconfig "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/jreleaser"

// Compatibility aliases preserve the established V1 JReleaser package API
// while canonical configuration ownership lives in internal/releasetool.
type Config = jreleaserconfig.Config
type Project = jreleaserconfig.Project
type ProjectLinks = jreleaserconfig.ProjectLinks
type ProjectLanguages = jreleaserconfig.ProjectLanguages
type JavaLanguage = jreleaserconfig.JavaLanguage
type Release = jreleaserconfig.Release
type GithubRelease = jreleaserconfig.GithubRelease
type Changelog = jreleaserconfig.Changelog
type Contributors = jreleaserconfig.Contributors
type ChangelogAppend = jreleaserconfig.ChangelogAppend
type Labeler = jreleaserconfig.Labeler
type Category = jreleaserconfig.Category

func LoadConfig() (*Config, error) {
	return jreleaserconfig.LoadConfig()
}

func LoadConfigAt(repositoryRoot string) (*Config, error) {
	return jreleaserconfig.LoadConfigAt(repositoryRoot)
}

func SaveConfig(config *Config) error {
	return jreleaserconfig.SaveConfig(config)
}

func SaveConfigAt(repositoryRoot string, config *Config) error {
	return jreleaserconfig.SaveConfigAt(repositoryRoot, config)
}
