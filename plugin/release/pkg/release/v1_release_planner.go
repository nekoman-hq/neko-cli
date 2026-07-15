//nolint:staticcheck // This file is the explicit boundary for deprecated V1 compatibility data.
package release

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type V1ReleasePlanningRequest struct {
	Intent    V1ReleaseIntent
	LatestTag string
}

type v1ReleasePlanner interface {
	Plan(V1ReleasePlanningRequest) (V1ReleasePlan, error)
}

type pureV1ReleasePlanner struct{}

// PlanV1Release constructs the complete V1 release plan without reading the
// environment, touching files, invoking Git, or selecting another source path.
func PlanV1Release(request V1ReleasePlanningRequest) (V1ReleasePlan, error) {
	return pureV1ReleasePlanner{}.Plan(request)
}

func (pureV1ReleasePlanner) Plan(request V1ReleasePlanningRequest) (V1ReleasePlan, error) {
	cfg := request.Intent.Config
	if cfg == nil {
		return V1ReleasePlan{}, fmt.Errorf("release configuration is missing")
	}
	current, err := semver.NewVersion(cfg.Version)
	if err != nil {
		return V1ReleasePlan{}, fmt.Errorf(
			"version %s in .release.neko.json is not a valid semantic version",
			cfg.Version,
		)
	}

	ignoredLatestTag := ""
	latestVersion := ""
	latest, latestErr := semver.NewVersion(request.LatestTag)
	if latestErr != nil {
		ignoredLatestTag = request.LatestTag
	} else if current.LessThan(latest) {
		return V1ReleasePlan{}, fmt.Errorf(
			"version violation: Local version %s is smaller than latest tag %s",
			current,
			latest,
		)
	} else {
		latestVersion = latest.String()
	}

	next := NextVersion(current, request.Intent.ReleaseType)
	return V1ReleasePlan{
		RepositoryRoot:    request.Intent.RepositoryRoot,
		UnitID:            request.Intent.Unit.ID,
		CurrentVersion:    current.String(),
		LatestVersion:     latestVersion,
		NextVersion:       next.String(),
		Tag:               "v" + next.String(),
		CommitMessage:     fmt.Sprintf("chore(neko-release): %s", next.String()),
		ConfigFile:        releaseconfig.V1FileName,
		Executor:          string(cfg.ReleaseSystem),
		ReleaseType:       request.Intent.ReleaseType,
		IgnoredLatestTag:  ignoredLatestTag,
		materializedFiles: []string{releaseconfig.V1FileName},
	}, nil
}
