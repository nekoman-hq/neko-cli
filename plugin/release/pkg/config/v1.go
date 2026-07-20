// Package config is mainly for the implementation of .release.neko.json
package config

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

// V1ProjectType is the legacy V1 project type.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
type V1ProjectType string

// V1ReleaseSystem is the legacy V1 release executor setting.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
type V1ReleaseSystem string

const (
	V1ProjectTypeFrontend V1ProjectType = "frontend"
	V1ProjectTypeBackend  V1ProjectType = "backend"
	V1ProjectTypeOther    V1ProjectType = "other"
)

const (
	V1ReleaseTypeReleaseIt  V1ReleaseSystem = V1ReleaseSystem(releasetool.ReleaseIt)
	V1ReleaseTypeJReleaser  V1ReleaseSystem = V1ReleaseSystem(releasetool.JReleaser)
	V1ReleaseTypeGoReleaser V1ReleaseSystem = V1ReleaseSystem(releasetool.GoReleaser)
)

// V1ReleaseConfig is the legacy .release.neko.json schema.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
type V1ReleaseConfig struct {
	ProjectName   string          `json:"project-name"`
	ProjectOwner  string          `json:"project-owner"`
	ProjectType   V1ProjectType   `json:"project-type"`
	ReleaseSystem V1ReleaseSystem `json:"release-system"`
	Version       string          `json:"version"`
	// TagName 	  string 		`json:"tag-name"`   (No implementation yet)
	// TokenName	  string		`json:"token-name"`	(No implementation yet)
}

// IsValid reports whether the legacy project type is supported.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
func (p V1ProjectType) IsValid() bool {
	switch p {
	case V1ProjectTypeFrontend, V1ProjectTypeBackend, V1ProjectTypeOther:
		return true
	default:
		return false
	}
}

// IsValid reports whether the legacy release system is supported.
//
// Deprecated: V1 is supported only as the legacy compatibility format.
func (r V1ReleaseSystem) IsValid() bool {
	return releasetool.Identity(r).Valid()
}
