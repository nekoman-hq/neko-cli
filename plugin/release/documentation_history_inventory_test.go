package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseDocumentationHistoryInventory(t *testing.T) {
	root := repositoryRoot(t)
	candidates := []struct {
		path           string
		classification string
		contains       string
	}{
		{"plugin/release/docs/history/001-behavior-preserving-refactor.md", "move", "closed behavior-preserving Release Plugin refactor"},
		{"plugin/release/docs/history/002-post-refactor-architecture-review.md", "split", "code-quality refactor completed in July 2026"},
		{"plugin/release/docs/history/003-post-refactor-architecture-evolution.md", "split", "records durable Release Plugin architecture decisions"},
		{"plugin/release/docs/architecture/current-state.md", "retain-current", "Release Plugin as it exists"},
		{"plugin/release/docs/architecture/maintainability-policy.md", "retain-current", "applies to new or materially changed Release Plugin production code"},
		{"plugin/release/docs/architecture/compatibility-notes.md", "retain-current", "Preserved contracts"},
		{"plugin/release/docs/architecture/v1-compatibility-policy.md", "retain-current", "V1 compatibility policy support decision"},
		{"docs/release/bootstrap-product-boundary.md", "retain-current", "defines the product boundary"},
		{"docs/release/compatibility.md", "retain-current", "Release Compatibility"},
		{"docs/release/github-actions-golden-path.md", "exclude-nonhistorical", "canonical, build-system-neutral guide"},
	}

	allowed := map[string]bool{
		"move": true, "split": true, "retain-current": true,
		"merge-duplicate": true, "mark-superseded": true,
		"remove-after-preservation": true, "exclude-nonhistorical": true,
	}
	for _, candidate := range candidates {
		candidate := candidate
		t.Run(filepath.Base(candidate.path), func(t *testing.T) {
			if !allowed[candidate.classification] {
				t.Fatalf("classification %q is outside the closed inventory vocabulary", candidate.classification)
			}
			content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(candidate.path)))
			if err != nil {
				t.Fatalf("read candidate %s: %v", candidate.path, err)
			}
			if !strings.Contains(string(content), candidate.contains) {
				t.Errorf("candidate %s no longer contains inventory evidence %q", candidate.path, candidate.contains)
			}
		})
	}

	formerSources := map[string]string{
		"plugin/release/docs/architecture/refactor-plan.md":          "plugin/release/docs/history/001-behavior-preserving-refactor.md",
		"plugin/release/docs/architecture/refactor-history.md":       "plugin/release/docs/history/001-behavior-preserving-refactor.md",
		"plugin/release/docs/architecture/post-refactor-review.md":   "plugin/release/docs/history/002-post-refactor-architecture-review.md",
		"plugin/release/docs/architecture/post-refactor-roadmap.md":  "plugin/release/docs/history/003-post-refactor-architecture-evolution.md",
		"plugin/release/docs/architecture/architecture-evolution.md": "plugin/release/docs/history/003-post-refactor-architecture-evolution.md",
	}
	for former, replacement := range formerSources {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(former))); !os.IsNotExist(err) {
			t.Errorf("former historical source %s unexpectedly exists: %v", former, err)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(replacement))); err != nil {
			t.Errorf("replacement for former source %s is missing: %v", former, err)
		}
	}
}

func TestCanonicalReleaseDocumentationRemainsCurrent(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range []string{
		"README.md",
		"docs/cli-reference.md",
		"docs/installation.md",
		"docs/plugins/release.md",
		"docs/release/cli-reference.md",
		"docs/release/overview.md",
		"docs/release/migration-v1-to-v2.md",
		"plugin/release/docs/architecture/current-state.md",
		"plugin/release/docs/architecture/package-ownership.md",
		"plugin/release/docs/architecture/architecture-decisions.md",
		"plugin/release/docs/architecture/maintainability-policy.md",
		"plugin/release/docs/architecture/compatibility-notes.md",
		"plugin/release/docs/architecture/v1-compatibility-policy.md",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("canonical current documentation %s is missing: %v", path, err)
			continue
		}
		if !info.Mode().IsRegular() {
			t.Errorf("canonical current documentation %s is not a regular file", path)
		}
	}
}
