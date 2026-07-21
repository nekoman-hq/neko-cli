package release

import "github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"

// configuredReleaseLifecycleStages returns a fresh immutable description of
// the direct-call root lifecycle. It is descriptive only and owns no handlers.
func configuredReleaseLifecycleStages() []pipelineinspection.LifecycleStage {
	return []pipelineinspection.LifecycleStage{}
}
