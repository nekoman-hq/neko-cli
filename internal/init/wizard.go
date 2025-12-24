package init

import "github.com/nekoman-hq/neko-cli/internal/config"

func runWizard() config.NekoConfig {
	cfg := config.NekoConfig{}

	askProjectType(&cfg)
	askReleaseSystem(&cfg)
	askInitialVersion(&cfg)

	config.Validate(&cfg)
	return cfg
}
