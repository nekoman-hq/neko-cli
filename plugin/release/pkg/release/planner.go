package release

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// VersionPlan is a non-mutating release version plan for one unit.
type VersionPlan struct {
	UnitID           string
	CurrentVersion   string
	NextVersion      string
	Tag              string
	Executor         string
	Delivery         string
	WorkingDirectory string
}

// PlanUnitVersionBump calculates the next version and tag without writing
// state, creating tags, or invoking executors.
func PlanUnitVersionBump(unit releaseconfig.ReleaseUnit, releaseType Type) (*VersionPlan, error) {
	current, err := semver.NewVersion(unit.Version)
	if err != nil {
		return nil, fmt.Errorf("unit %q version %q is not valid SemVer", unit.ID, unit.Version)
	}
	next := NextVersion(current, releaseType)
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return nil, err
	}

	return &VersionPlan{
		UnitID:           unit.ID,
		CurrentVersion:   current.String(),
		NextVersion:      next.String(),
		Tag:              tagSpec.Format(next.String()),
		Executor:         unit.ExecutorType,
		Delivery:         unit.Delivery,
		WorkingDirectory: unit.WorkingDirectory,
	}, nil
}
