package init

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      17.12.2025
*/

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
)

func Run() {
	// Check if config already exists
	if _, err := os.Stat(".neko.json"); err == nil {
		var overwrite bool
		confirm := &survey.Confirm{
			Message: ".neko.json already exists. Overwrite it?",
			Default: false,
		}

		if err := survey.AskOne(confirm, &overwrite); err != nil {
			errors.Warning(
				"Initialization cancelled",
				"Configuration wizard was aborted by the user.",
			)
			return
		}

		if !overwrite {
			errors.Warning(
				"Initialization aborted",
				"Existing .neko.json was not overwritten.",
			)
			return
		}
	}

	var projectTypeInput string
	var releaseTypeInput string

	cfg := config.NekoConfig{}

	// Project type
	if err := survey.AskOne(&survey.Select{
		Message: "What kind of project is this?",
		Options: []string{
			string(config.ProjectTypeFrontend),
			string(config.ProjectTypeBackend),
			string(config.ProjectTypeOther),
		},
	}, &projectTypeInput); err != nil {
		errors.Error(
			"Project type selection failed",
			"Could not read project type input.",
			errors.ErrSurveyFailed,
		)
		return
	}

	cfg.ProjectType = config.ProjectType(projectTypeInput)
	if !cfg.ProjectType.IsValid() {
		errors.Error(
			"Invalid project type",
			"Selected project type is not supported.",
			errors.ErrConfigMarshal,
		)
		return
	}

	// Release system
	if err := survey.AskOne(&survey.Select{
		Message: "Which release system should be used?",
		Options: getReleaseOptions(cfg.ProjectType),
	}, &releaseTypeInput); err != nil {
		errors.Error(
			"Release system selection failed",
			"Could not read release system input.",
			errors.ErrSurveyFailed,
		)
		return
	}

	cfg.ReleaseSystem = config.ReleaseType(releaseTypeInput)
	if !cfg.ReleaseSystem.IsValid() {
		errors.Error(
			"Invalid release system",
			"Selected release system is not supported.",
			errors.ErrConfigMarshal,
		)
		return
	}

	// Initial version
	if err := survey.AskOne(&survey.Input{
		Message: "Initial version:",
		Default: "0.1.0",
		Help:    "Semantic Versioning (MAJOR.MINOR.PATCH)",
	}, &cfg.Version); err != nil {
		errors.Error(
			"Version input failed",
			"Could not read version input.",
			errors.ErrSurveyFailed,
		)
		return
	}

	config.Validate(&cfg)

	if err := config.SaveConfig(cfg); err != nil {
		errors.Fatal(
			"Configuration write failed",
			err.Error(),
			errors.ErrConfigWrite,
		)
		return
	}

	printSetupInstructions(cfg)
}

func getReleaseOptions(projectType config.ProjectType) []string {
	switch projectType {
	case config.ProjectTypeFrontend:
		return []string{string(config.ReleaseTypeReleaseIt)}
	case config.ProjectTypeBackend:
		return []string{string(config.ReleaseTypeJReleaser)}
	case config.ProjectTypeOther:
		return []string{string(config.ReleaseTypeGoReleaser)}
	default:
		return []string{}
	}
}

func printSetupInstructions(cfg config.NekoConfig) {
	println("\n✓ .neko.json created successfully\n")
	println("Next steps:")
	println("  • Use 'neko release' to create a release")
	println("  • Neko automatically manages the version in:")

	switch cfg.ReleaseSystem {
	case config.ReleaseTypeReleaseIt:
		println("    - package.json")
		println("    - .release-it.json")

	case config.ReleaseTypeJReleaser:
		println("    - jreleaser.yml")
		println("    - pom.xml / build.gradle")

	case config.ReleaseTypeGoReleaser:
		println("    - .goreleaser.yml")
		println("    - Git tags")
	}

	println("\nTip: The version in .neko.json is the single source of truth.")
}
