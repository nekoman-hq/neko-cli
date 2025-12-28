// Package releaseit provides functions for release automation.
package releaseit

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type ReleaseIt struct {
	release.ToolBase
}

func (r *ReleaseIt) Name() string {
	return "release-it"
}

func (r *ReleaseIt) Init(cfg *config.NekoConfig) error {
	// assert package.json
	// assert node installed
	// install release it
	// npm init release-it
	// validate with npx release-it version

	log.V(log.Init, fmt.Sprintf("Initializing %s for project %s@%s",
		log.ColorText(log.ColorGreen, r.Name()),
		cfg.ProjectName,
		cfg.Version,
	))

	log.Print(log.Init, "\uF00C Initialization complete for %s", log.ColorText(log.ColorCyan, r.Name()))

	return nil
}

func (r *ReleaseIt) Release(v *semver.Version) error {
	//

	return nil
}

func (r *ReleaseIt) Survey(v *semver.Version) (release.Type, error) {
	return release.Patch, nil
}

func (r *ReleaseIt) SupportsSurvey() bool {
	return true
}

func init() {
	release.Register(&ReleaseIt{})
}
