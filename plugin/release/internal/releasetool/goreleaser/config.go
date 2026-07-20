package goreleaser

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Build struct {
	ID     string   `yaml:"id"`
	Binary string   `yaml:"binary"`
	Main   string   `yaml:"main"`
	Goos   []string `yaml:"goos"`
}

type FormatOverride struct {
	Goos    string   `yaml:"goos"`
	Formats []string `yaml:"formats"`
}

type Archive struct {
	ID              string           `yaml:"id"`
	IDs             []string         `yaml:"ids"`
	Formats         []string         `yaml:"formats"`
	NameTemplate    string           `yaml:"name_template"`
	FormatOverrides []FormatOverride `yaml:"format_overrides"`
}

type Checksum struct {
	NameTemplate string `yaml:"name_template"`
}

type Release struct {
	IDs []string `yaml:"ids"`
}

// Config is the focused GoReleaser v2 subset consumed by Release Plugin
// verification. Unknown GoReleaser fields remain intentionally tolerated.
//
//nolint:govet // Field order mirrors the focused GoReleaser YAML contract.
type Config struct {
	Version     int       `yaml:"version"`
	ProjectName string    `yaml:"project_name"`
	Builds      []Build   `yaml:"builds"`
	Archives    []Archive `yaml:"archives"`
	Checksum    *Checksum `yaml:"checksum"`
	Release     Release   `yaml:"release"`
}

// ParseConfig parses the focused GoReleaser fields while preserving the
// historical yaml.Unmarshal contract, including first-document handling and
// tolerance of unrelated GoReleaser fields.
func ParseConfig(content []byte) (Config, error) {
	var config Config
	if err := yaml.Unmarshal(content, &config); err != nil {
		return Config{}, fmt.Errorf("parse focused GoReleaser config: %w", err)
	}
	return config, nil
}

func BuildByID(builds []Build, id string) (Build, bool) {
	for _, build := range builds {
		if build.ID == id {
			return build, true
		}
	}
	return Build{}, false
}

func ArchiveByID(archives []Archive, id string) (Archive, bool) {
	for _, archive := range archives {
		if archive.ID == id {
			return archive, true
		}
	}
	return Archive{}, false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsAll(values []string, wants ...string) bool {
	for _, want := range wants {
		if !contains(values, want) {
			return false
		}
	}
	return true
}
