package config

//lint:file-ignore fieldalignment Canonical release models keep logical field order for readability and JSON-domain documentation

import (
	"fmt"
	"path/filepath"
)

// SourceFormat identifies which on-disk release configuration format was used.
type SourceFormat string

const (
	// SourceFormatV1 is the legacy .release.neko.json format.
	SourceFormatV1 SourceFormat = "v1"
	// SourceFormatV2 is the .neko/release.config.json plus state format.
	SourceFormatV2 SourceFormat = "v2"
)

// ReleaseRepository is the normalized internal view shared by V1 and V2.
// V1 is intentionally represented as one virtual "default" unit so legacy
// commands can continue to rely on the old file while newer code can reason
// about repositories and units without format-specific branching.
//
//nolint:govet // Logical domain order is clearer than fieldalignment ordering here.
type ReleaseRepository struct {
	RepositoryRoot string
	SchemaVersion  int
	Units          []ReleaseUnit
	SourceFormat   SourceFormat
	Legacy         *V1ReleaseConfig
}

// ReleaseUnit is the normalized releaseable unit used by both V1 and V2.
//
//nolint:govet // Logical domain order is clearer than fieldalignment ordering here.
type ReleaseUnit struct {
	ID               string
	DisplayName      string
	Paths            []string
	WorkingDirectory string
	TagPrefix        string
	ExecutorType     string
	Delivery         string
	Version          string
}

// NormalizeV1Repository converts the legacy global V1 config into the shared
// single-unit model without renaming or removing any legacy fields.
func NormalizeV1Repository(repositoryRoot string, cfg *V1ReleaseConfig) *ReleaseRepository {
	return &ReleaseRepository{
		RepositoryRoot: repositoryRoot,
		SchemaVersion:  1,
		SourceFormat:   SourceFormatV1,
		Legacy:         cfg,
		Units: []ReleaseUnit{
			{
				ID:               "default",
				DisplayName:      cfg.ProjectName,
				Paths:            []string{"**"},
				WorkingDirectory: ".",
				TagPrefix:        "v",
				ExecutorType:     string(cfg.ReleaseSystem),
				Delivery:         string(DeliveryLocal),
				Version:          cfg.Version,
			},
		},
	}
}

// LoadReleaseRepository loads and validates the configured release repository
// at repositoryRoot. V2 wins when present; otherwise the legacy V1 file is used.
func LoadReleaseRepository(repositoryRoot string) (*ReleaseRepository, error) {
	if repositoryRoot == "" {
		repositoryRoot = "."
	}

	if V2ConfigExists(repositoryRoot) {
		if V1ConfigExistsAt(repositoryRoot) {
			return nil, fmt.Errorf("release configuration conflict: %s and %s both exist at repository root", filepath.Join(repositoryRoot, V1FileName), V2ConfigPath(repositoryRoot))
		}
		return LoadV2Repository(repositoryRoot)
	}

	cfg, err := V1LoadConfigAt(filepath.Join(repositoryRoot, V1FileName))
	if err != nil {
		return nil, err
	}
	return NormalizeV1Repository(repositoryRoot, cfg), nil
}
