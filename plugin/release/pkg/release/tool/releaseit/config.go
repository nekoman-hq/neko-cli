package releaseit

import releaseitconfig "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/releaseit"

// Compatibility aliases preserve the established V1 release-it package API
// while canonical configuration ownership lives in internal/releasetool.
type Config = releaseitconfig.Config
type GithubRelease = releaseitconfig.GithubRelease
type GitConfig = releaseitconfig.GitConfig
type HooksConfig = releaseitconfig.HooksConfig

func LoadConfig() (*Config, error) {
	return releaseitconfig.LoadConfig()
}

func LoadConfigAt(repositoryRoot string) (*Config, error) {
	return releaseitconfig.LoadConfigAt(repositoryRoot)
}

func SaveConfig(config *Config) error {
	return releaseitconfig.SaveConfig(config)
}

func SaveConfigAt(repositoryRoot string, config *Config) error {
	return releaseitconfig.SaveConfigAt(repositoryRoot, config)
}

func InitDefaultConfig(projectName string) (*Config, error) {
	return releaseitconfig.InitDefaultConfig(projectName)
}
