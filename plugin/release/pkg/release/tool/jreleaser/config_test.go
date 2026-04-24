package jreleaser

import (
	"os"
	"strings"
	"testing"
)

func TestSaveConfigUsesJReleaserChangelogFieldNames(t *testing.T) {
	tempDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	if err = os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		if err = os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	includeLabels := []string{"feat"}
	labelers := []Labeler{{
		Label: "feat",
		Title: "regex:feat",
		Order: 1,
	}}
	categories := []Category{{
		Title:  "Features",
		Key:    "features",
		Labels: []string{"feat"},
		Order:  1,
	}}

	cfg := &Config{
		Project: Project{
			Languages: ProjectLanguages{
				Java: JavaLanguage{
					GroupID: "at.trainity",
					Version: "25",
				},
			},
			Links: ProjectLinks{
				Homepage: "https://trainity.net",
			},
			Name:          "Trainity",
			Version:       "0.2.0",
			License:       "Proprietary",
			InceptionYear: "2025",
		},
		Release: Release{
			Github: GithubRelease{
				Owner:       "nekoman-hq",
				Name:        "trainity-backend",
				TagName:     "v{{projectVersion}}",
				ReleaseName: "trainity@{{projectVersion}}",
				Changelog: Changelog{
					IncludeLabels: &includeLabels,
					Labelers:      &labelers,
					Categories:    &categories,
					Contributors: &Contributors{
						Enabled: false,
					},
					Append: &ChangelogAppend{
						Title:   "## [{{tagName}}]",
						Target:  "CHANGELOG.md",
						Enabled: true,
					},
					Sort:             "DESC",
					Formatted:        "ALWAYS",
					Preset:           "gitmoji",
					Enabled:          true,
					SkipMergeCommits: false,
				},
			},
		},
	}

	if err = SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	data, err := os.ReadFile("jreleaser.yml")
	if err != nil {
		t.Fatalf("read jreleaser.yml: %v", err)
	}

	content := string(data)

	for _, key := range []string{
		"includeLabels:",
		"skipMergeCommits:",
		"labelers:",
		"categories:",
		"contributors:",
		"append:",
	} {
		if !strings.Contains(content, key) {
			t.Fatalf("expected %q in saved config:\n%s", key, content)
		}
	}

	for _, key := range []string{
		"includelabels:",
		"skipmergecommits:",
	} {
		if strings.Contains(content, key) {
			t.Fatalf("did not expect legacy key %q in saved config:\n%s", key, content)
		}
	}
}

func TestSaveConfigOmitsNilOptionalChangelogFields(t *testing.T) {
	tempDir := t.TempDir()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	if err = os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	defer func() {
		if err = os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	}()

	cfg := &Config{
		Project: Project{
			Languages: ProjectLanguages{
				Java: JavaLanguage{
					GroupID: "at.trainity",
					Version: "25",
				},
			},
			Links: ProjectLinks{
				Homepage: "https://trainity.net",
			},
			Name:          "Trainity",
			Version:       "0.2.0",
			License:       "Proprietary",
			InceptionYear: "2025",
		},
		Release: Release{
			Github: GithubRelease{
				Owner:       "nekoman-hq",
				Name:        "trainity-backend",
				TagName:     "v{{projectVersion}}",
				ReleaseName: "trainity@{{projectVersion}}",
				Changelog: Changelog{
					Sort:             "DESC",
					Formatted:        "ALWAYS",
					Preset:           "gitmoji",
					Enabled:          true,
					SkipMergeCommits: false,
				},
			},
		},
	}

	if err = SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	data, err := os.ReadFile("jreleaser.yml")
	if err != nil {
		t.Fatalf("read jreleaser.yml: %v", err)
	}

	content := string(data)

	for _, key := range []string{
		"includeLabels:",
		"labelers:",
		"categories:",
		"contributors:",
		"append:",
		"authors:",
	} {
		if strings.Contains(content, key) {
			t.Fatalf("did not expect %q in saved config when unset:\n%s", key, content)
		}
	}
}
