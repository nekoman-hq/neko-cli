package config

//lint:file-ignore fieldalignment Canonical release models keep logical field order for readability and JSON-domain documentation

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"

const (
	V2Directory      = ".neko"
	V2ConfigFileName = "release.config.json"
	V2StateFileName  = "release.state.json"
)

// ExecutorType is a supported release executor identifier in V2 config.
type ExecutorType string

const (
	ExecutorJReleaser  ExecutorType = ExecutorType(releasetool.JReleaser)
	ExecutorReleaseIt  ExecutorType = ExecutorType(releasetool.ReleaseIt)
	ExecutorGoReleaser ExecutorType = ExecutorType(releasetool.GoReleaser)
)

// DeliveryType is the configured release delivery channel.
type DeliveryType string

const (
	DeliveryLocal         DeliveryType = "local"
	DeliveryGitHubActions DeliveryType = "github-actions"
)

// UnitKind optionally classifies a V2 release unit.
type UnitKind string

const (
	UnitKindPlugin UnitKind = "plugin"
)

// V2ReleaseConfig is the committed repository architecture file.
//
//nolint:govet // JSON-domain order mirrors the canonical config document.
type V2ReleaseConfig struct {
	SchemaVersion int      `json:"schemaVersion"`
	Units         []V2Unit `json:"units"`
}

// V2Unit configures one releaseable unit in a repository.
//
//nolint:govet // JSON-domain order mirrors the canonical config document.
type V2Unit struct {
	ID               string     `json:"id"`
	DisplayName      string     `json:"displayName,omitempty"`
	Paths            []string   `json:"paths"`
	WorkingDirectory string     `json:"workingDirectory,omitempty"`
	TagPrefix        string     `json:"tagPrefix"`
	Kind             UnitKind   `json:"kind,omitempty"`
	Plugin           *V2Plugin  `json:"plugin,omitempty"`
	Executor         V2Executor `json:"executor"`
}

// V2Plugin configures plugin-registry metadata for a V2 plugin unit.
type V2Plugin struct {
	Name        string `json:"name"`
	Manifest    string `json:"manifest"`
	AssetPrefix string `json:"assetPrefix"`
	BinaryName  string `json:"binaryName"`
}

// V2Executor configures the release executor and delivery channel for a unit.
type V2Executor struct {
	Type     ExecutorType `json:"type"`
	Delivery DeliveryType `json:"delivery,omitempty"`
	Workflow string       `json:"workflow,omitempty"`
}

// V2ReleaseState is the version source of truth for all configured V2 units.
//
//nolint:govet // JSON-domain order mirrors the canonical state document.
type V2ReleaseState struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Units         map[string]V2UnitState `json:"units"`
}

// V2UnitState stores mutable release state for one V2 unit.
type V2UnitState struct {
	Version string `json:"version"`
}

// IsValid reports whether the executor is supported by V2.
func (e ExecutorType) IsValid() bool {
	return releasetool.Identity(e).Valid()
}

// IsValid reports whether the delivery type is a recognized release delivery
// value. V2 executable configs currently support github-actions only; local is
// retained as a known value so legacy V1 data and invalid V2 configs can be
// reported clearly.
func (d DeliveryType) IsValid() bool {
	switch d {
	case DeliveryLocal, DeliveryGitHubActions:
		return true
	default:
		return false
	}
}
