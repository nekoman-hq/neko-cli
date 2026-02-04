// Package config is mainly for the implementation of .release.neko.json
package config

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

type (
	ProjectType   string
	ReleaseSystem string
)

const (
	ProjectTypeFrontend ProjectType = "frontend"
	ProjectTypeBackend  ProjectType = "backend"
	ProjectTypeOther    ProjectType = "other"
)

const (
	ReleaseTypeReleaseIt  ReleaseSystem = "release-it"
	ReleaseTypeJReleaser  ReleaseSystem = "jreleaser"
	ReleaseTypeGoReleaser ReleaseSystem = "goreleaser"
)

type NekoConfig struct {
	ProjectName   string        `json:"project-name"`
	ProjectOwner  string        `json:"project-owner"`
	ProjectType   ProjectType   `json:"project-type"`
	ReleaseSystem ReleaseSystem `json:"release-system"`
	Version       string        `json:"version"`
	// TagName 	  string 		`json:"tag-name"`   (No implementation yet)
	// TokenName	  string		`json:"token-name"`	(No implementation yet)
}

func (p ProjectType) IsValid() bool {
	switch p {
	case ProjectTypeFrontend, ProjectTypeBackend, ProjectTypeOther:
		return true
	default:
		return false
	}
}

func (r ReleaseSystem) IsValid() bool {
	switch r {
	case ReleaseTypeReleaseIt, ReleaseTypeJReleaser, ReleaseTypeGoReleaser:
		return true
	default:
		return false
	}
}
