// Package jreleaser includes the jreleaser release-system logic
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
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type JReleaser struct {
	release.ToolBase

	State struct {
		PreHead string

		ReleaseCommitHash string
		PushedCommit      bool

		TagName     string
		RanJRelease bool
	}
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

	if err := j.RequireBinary(j.Name()); err != nil {
		return err
	}
	if err := j.runJReleaserInit(cfg); err != nil {
		return err
	}
	if err := j.runJReleaserCheck(); err != nil {
		return err
	}

	log.Print(log.Init, "\uF00C Initialization complete for %s", log.ColorText(log.ColorCyan, j.Name()))
	return nil
}

func (j *JReleaser) Release(v *semver.Version) error {
	pre, err := git.Head()

	if err != nil {
		return err
	}
	j.State.PreHead = pre

	if err := j.syncJReleaser(v); err != nil {
		return err
	}

	if err := j.CreateReleaseCommit(v); err != nil {
		return err
	}

	head, err := git.Head()
	if err != nil {
		return err
	}
	j.State.ReleaseCommitHash = head

	if err := j.PushCommits(); err != nil {
		return err
	}
	j.State.PushedCommit = true

	if err := j.runJReleaserDryRun(); err != nil {
		return err
	}

	if err := j.runJReleaserRelease(); err != nil {
		return err
	}
	j.State.TagName = fmt.Sprintf("v%s", v.String())
	j.State.RanJRelease = true

	return nil
}

func (j *JReleaser) RevertRelease() error {
	return j.RevertGitRelease(release.GitReleaseState{
		PreHead:              j.State.PreHead,
		ReleaseHead:          j.State.ReleaseCommitHash,
		PushedCommit:         j.State.PushedCommit,
		TagName:              j.State.TagName,
		PushedTag:            j.State.RanJRelease,
		GitHubReleaseTag:     j.State.TagName,
		CreatedGitHubRelease: j.State.RanJRelease,
	})
}

func (j *JReleaser) Survey(v *semver.Version) (release.Type, error) {
	return release.NekoSurvey(v)
}

func (j *JReleaser) SupportsSurvey() bool {
	return true
}

func (j *JReleaser) runJReleaserInit(cfg *config.NekoConfig) error {
	log.V(log.Init, "Generating JReleaser configuration...")

	if _, err := os.Stat("jreleaser.yml"); err == nil {
		log.Print(
			log.Init,
			"Skipping jreleaser init, %s already exists",
			log.ColorText(log.ColorCyan, "jreleaser.yml"),
		)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf(
			"Failed to check jreleaser.yml: %w", err,
		)
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
				TagName:     fmt.Sprintf("v{{projectVersion}}"),
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
		return fmt.Errorf(
			"Configuration write failed: %w", err,
		)
	}
	log.Print(log.Init, "\uF00C JReleaser configuration generated for %s", log.ColorText(log.ColorCyan, cfg.ProjectName))

	return nil
}

func (j *JReleaser) runJReleaserCheck() error {
	log.V(log.Init,
		"Checking JReleaser configuration: %s",
		log.ColorText(log.ColorGreen, "jreleaser config"),
	)

	output, err := executeJReleaserCommand("config")
	if err != nil {
		return fmt.Errorf(
			"JReleaser configuration check failed: %s: %w", string(output), err,
		)
	}

	log.Print(
		log.Init,
		"\uF00C Configuration check passed for %s",
		log.ColorText(log.ColorCyan, "jreleaser"),
	)

	return nil
}

func (j *JReleaser) syncJReleaser(v *semver.Version) error {
	log.V(log.Release,
		fmt.Sprintf("Syncing JReleaser configuration with version %s",
			log.ColorText(log.ColorCyan, v.String()),
		),
	)

	if _, err := os.Stat("jreleaser.yml"); os.IsNotExist(err) {
		return fmt.Errorf("jreleaser.yml not found")
	}

	jcfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf(
			"Configuration serialization failed: %w", err,
		)
	}

	jcfg.Project.Version = v.String()

	if err := SaveConfig(jcfg); err != nil {
		return fmt.Errorf(
			"Configuration write failed: %w", err,
		)
	}

	log.Print(log.Release,
		"\uF00C JReleaser version updated to %s",
		log.ColorText(log.ColorGreen, v.String()),
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
		"\uF00C JReleaser dry run %s",
		log.ColorText(log.ColorGreen, "successful"),
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
		return fmt.Errorf(
			"JReleaser release failed: %s: %w", string(output), err,
		)
	}

	log.Print(
		log.Release,
		"\uF00C JReleaser release %s",
		log.ColorText(log.ColorGreen, "successful"),
	)
	return nil
}

func executeJReleaserCommand(action string) ([]byte, error) {
	pat, err := config.GetPAT()
	if err != nil {
		return nil, err
	}

	maskedPat := strings.Repeat("*", 5)
	log.V(log.Init, fmt.Sprintf("Executing command: JRELEASER_GITHUB_TOKEN=%s jreleaser %s", maskedPat, action))

	cmd := exec.Command("jreleaser", action)
	cmd.Env = append(os.Environ(), "JRELEASER_GITHUB_TOKEN="+pat)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("failed to execute command: %w", err)
	}

	return output, nil
}

func init() {
	release.Register(&JReleaser{})
}
