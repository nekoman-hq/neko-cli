package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      23.12.2025
*/

import "github.com/nekoman-hq/neko-cli/internal/config"

func runWizard() (config.NekoConfig, error) {
	cfg := config.NekoConfig{}

	askProjectType(&cfg)
	askReleaseSystem(&cfg)
	askInitialVersion(&cfg)

	err := config.Validate(&cfg)
	if err != nil {
		return config.NekoConfig{}, err
	}

	return cfg, nil
}
