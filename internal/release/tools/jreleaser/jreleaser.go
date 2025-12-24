package jreleaser

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      24.12.2025
*/

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nekoman-hq/neko-cli/internal/config"
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/release"
)

type Jreleaser struct {
	release.ToolBase
}

func (j *Jreleaser) Name() string {
	return "jreleaser"
}

func (j *Jreleaser) Init(v *semver.Version, cfg *config.NekoConfig) error {

	j.RequireBinary(j.Name())
	runJreleaserInit(cfg)
	return nil
}

func (j *Jreleaser) Release(v *semver.Version) error {
	return nil
}

func (j *Jreleaser) Survey(v *semver.Version) (release.Type, error) {
	return release.NekoSurvey(v)
}

func (j *Jreleaser) SupportsSurvey() bool {
	return true
}

func runJreleaserInit(cfg *config.NekoConfig) {
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
		return
	}
}

func init() {
	release.Register(&Jreleaser{})
}
