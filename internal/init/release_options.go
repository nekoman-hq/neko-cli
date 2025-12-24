package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      23.12.2025
*/

import "github.com/nekoman-hq/neko-cli/internal/config"

func releaseOptionsFor(projectType config.ProjectType) []string {
	switch projectType {
	case config.ProjectTypeFrontend:
		return []string{string(config.ReleaseTypeReleaseIt)}
	case config.ProjectTypeBackend:
		return []string{string(config.ReleaseTypeJReleaser)}
	case config.ProjectTypeOther:
		return []string{string(config.ReleaseTypeGoReleaser)}
	default:
		return nil
	}
}
