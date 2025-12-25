package jreleaser

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      24.12.2025
*/

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type JReleaser struct {
	release.ToolBase
}

func (j *JReleaser) Name() string {
	return "jreleaser"
}

func (j *JReleaser) Init(cfg *config.NekoConfig) error {

	log.V(log.Init, fmt.Sprintf("Initializing %s for project %s@%s",
		log.ColorText(log.ColorGreen, j.Name()),
		cfg.ProjectName,
		cfg.Version,
	))

	j.RequireBinary(j.Name())
	j.runJReleaserInit(cfg)
	j.runJReleaserCheck()

	log.Print(log.Init, fmt.Sprintf("\uF00C Initialization complete for %s", log.ColorText(log.ColorCyan, j.Name())))
	return nil
}

func (j *JReleaser) Release(v *semver.Version) error {

	if err := j.syncJReleaser(v); err != nil {
		return err
	}

	if err := j.ToolBase.CreateReleaseCommit(v); err != nil {
		return err
	}

	if err := j.ToolBase.PushCommits(); err != nil {
		return err
	}

	if err := j.runJReleaserDryRun(); err != nil {
		return err
	}

	if err := j.runJReleaserRelease(); err != nil {
		return err
	}

	return nil
}

func (j *JReleaser) Survey(v *semver.Version) (release.Type, error) {
	return release.NekoSurvey(v)
}

func (j *JReleaser) SupportsSurvey() bool {
	return true
}

func (j *JReleaser) runJReleaserInit(cfg *config.NekoConfig) {
	log.V(log.Init, "Generating JReleaser configuration...")

	if _, err := os.Stat(".jreleaser.yml"); err == nil {
		log.Print(
			log.Init,
			fmt.Sprintf(
				"Skipping jreleaser init, %s already exists",
				log.ColorText(log.ColorCyan, "jreleaser.yml"),
			),
		)
		return
	} else if !os.IsNotExist(err) {
		errors.Fatal(
			"Failed to check goreleaser.yml",
			err.Error(),
			errors.ErrFileAccess,
		)
		return
	}

	jcfg := &Config{
		Project: Project{
			Name:    cfg.ProjectName,
			Version: cfg.Version,
			Authors: []string{"Authors here..."},
			License: "Proprietary",
			Languages: ProjectLanguages{
				Java: JavaLanguage{
					GroupID: fmt.Sprintf("at.%s", cfg.ProjectName),
					Version: "25",
				},
			},
			InceptionYear: strconv.Itoa(time.Now().Year()),
		},
		Release: Release{
			Github: GithubRelease{
				Overwrite:   false,
				Owner:       cfg.ProjectOwner,
				Name:        cfg.ProjectName,
				TagName:     fmt.Sprintf("%s@{{projectVersion}}", cfg.ProjectName),
				ReleaseName: fmt.Sprintf("%s@{{projectVersion}}", cfg.ProjectName),
				Changelog: Changelog{
					Enabled:          true,
					Sort:             "DESC",
					SkipMergeCommits: true,
					Formatted:        "ALWAYS",
					Preset:           "gitmoji",
					Contributors: Contributors{
						Enabled: false,
					},
					Append: ChangelogAppend{
						Enabled: true,
						Title:   "## [{{tagName}}]",
						Target:  "CHANGELOG.md",
					},
					IncludeLabels: []string{
						"feature", "feat", "fix", "refactor", "improvement", "chore", "test", "docs", "hotfix",
					},
					Labelers: []Labeler{
						{Label: "feat", Title: "regex:feat", Order: 1},
						{Label: "feature", Title: "regex:feature", Order: 1},
						{Label: "fix", Title: "regex:fix", Order: 2},
						{Label: "bug", Title: "regex:bug", Order: 2},
						{Label: "refactor", Title: "regex:refactor", Order: 3},
						{Label: "improvement", Title: "regex:improvement", Order: 3},
						{Label: "docs", Title: "regex:docs", Order: 4},
						{Label: "chore", Title: "regex:chore", Order: 5},
						{Label: "test", Title: "regex:test", Order: 6},
						{Label: "hotfix", Title: "regex:hotfix", Order: 7},
					},
					Categories: []Category{
						{Title: "Features", Key: "features", Labels: []string{"feat", "feature"}, Order: 1},
						{Title: "Bug Fixes", Key: "fixes", Labels: []string{"fix", "bug"}, Order: 2},
						{Title: "Refactoring", Key: "refactor", Labels: []string{"refactor", "improvement"}, Order: 3},
						{Title: "Documentation", Key: "docs", Labels: []string{"docs"}, Order: 4},
						{Title: "Chores", Key: "chore", Labels: []string{"chore"}, Order: 5},
						{Title: "Tests", Key: "test", Labels: []string{"test"}, Order: 6},
						{Title: "Hotfixes", Key: "hotfix", Labels: []string{"hotfix"}, Order: 7},
					},
				},
			},
		},
	}

	if err := SaveConfig(jcfg); err != nil {
		errors.Fatal(
			"Configuration write failed",
			err.Error(),
			errors.ErrConfigWrite,
		)
	}
	log.Print(log.Init, fmt.Sprintf("\uF00C JReleaser configuration generated for %s", log.ColorText(log.ColorCyan, cfg.ProjectName)))
}

func (j *JReleaser) runJReleaserCheck() {
	log.V(log.Init,
		fmt.Sprintf("Checking JReleaser configuration: %s",
			log.ColorText(log.ColorGreen, "jreleaser config"),
		),
	)

	output, err := executeJReleaserCommand("config")
	if err != nil {
		errors.Fatal(
			"JReleaser configuration check failed",
			fmt.Sprintf("Command failed: %s\nOutput: %s", err.Error(), string(output)),
			errors.ErrDependencyMissing,
		)
	}

	log.Print(
		log.Init,
		fmt.Sprintf(
			"\uF00C Configuration check passed for %s",
			log.ColorText(log.ColorCyan, "jreleaser"),
		),
	)
}

func (j *JReleaser) syncJReleaser(v *semver.Version) error {
	log.V(log.Release,
		fmt.Sprintf("Syncing JReleaser configuration with version %s",
			log.ColorText(log.ColorCyan, v.String()),
		),
	)

	if _, err := os.Stat(".jreleaser.yml"); os.IsNotExist(err) {
		return fmt.Errorf(".jreleaser.yml not found")
	}

	jcfg, err := LoadConfig()
	if err != nil {
		errors.Fatal(
			"Configuration serialization failed",
			"Could not marshal jreleaser.yml: "+err.Error(),
			errors.ErrConfigMarshal,
		)
		return err
	}

	jcfg.Project.Version = v.String()

	if err := SaveConfig(jcfg); err != nil {
		errors.Fatal(
			"Configuration write failed",
			"Could not write jreleaser.yml: "+err.Error(),
			errors.ErrConfigWrite,
		)
	}

	log.Print(log.Release,
		fmt.Sprintf("\uF00C JReleaser version updated to %s",
			log.ColorText(log.ColorGreen, v.String()),
		),
	)

	return nil
}

// runJReleaserDryRun executes JReleaser in dry-run mode
func (j *JReleaser) runJReleaserDryRun() error {
	action := "full-release --dry-run"

	log.V(
		log.Release,
		fmt.Sprintf(
			"Running JReleaser dry run: %s",
			log.ColorText(log.ColorGreen, "jreleaser "+action),
		),
	)

	output, err := executeJReleaserCommand(action)
	if err != nil {
		errors.Warning(
			"JReleaser dry run failed",
			fmt.Sprintf(
				"This is a warning - proceeding anyway: %s",
				strings.TrimSpace(string(output)),
			),
		)
		log.Print(log.Release, "\u26A0 Dry run failed, but continuing with release")
		return nil
	}

	log.Print(
		log.Release,
		fmt.Sprintf(
			"\uF00C JReleaser dry run %s",
			log.ColorText(log.ColorGreen, "successful"),
		),
	)
	return nil
}

// runJReleaserRelease executes the full jreleaser release
func (j *JReleaser) runJReleaserRelease() error {
	action := "full-release"

	log.V(
		log.Release,
		fmt.Sprintf(
			"Running JReleaser release: %s",
			log.ColorText(log.ColorGreen, "jreleaser "+action),
		),
	)

	output, err := executeJReleaserCommand(action)
	if err != nil {
		errors.Fatal(
			"JReleaser release failed",
			fmt.Sprintf(
				"jreleaser full-release failed: %s",
				strings.TrimSpace(string(output)),
			),
			errors.ErrJReleaserExecution,
		)
	}

	log.Print(
		log.Release,
		fmt.Sprintf(
			"\uF00C JReleaser release %s",
			log.ColorText(log.ColorGreen, "successful"),
		),
	)
	return nil
}

func executeJReleaserCommand(action string) ([]byte, error) {
	pat := config.GetPAT()
	if pat == "" {
		return nil, fmt.Errorf("personal access token is empty")
	}

	cmdStr := fmt.Sprintf("JRELEASER_GITHUB_TOKEN=%s jreleaser %s", pat, action)

	maskedPat := strings.Repeat("*", 5)
	log.V(log.Init, fmt.Sprintf("Executing command: JRELEASER_GITHUB_TOKEN=%s jreleaser %s", maskedPat, action))

	cmd := exec.Command("sh", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute command: %w", err)
	}

	return output, nil
}

func init() {
	release.Register(&JReleaser{})
}
