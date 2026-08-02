package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var mergedOrRemovedDocumentation = []string{
	"docs/plugins/plugin-deploy.md",
	"docs/plugins/plugin-monitoring.md",
	"docs/release/bootstrap-product-boundary.md",
	"docs/release/dispatch-contract.md",
	"docs/release/dispatch-journal.md",
	"docs/release/execution-journal.md",
	"docs/release/executors.md",
	"docs/release/git-coordination.md",
	"docs/release/github-actions-dispatch.md",
	"docs/release/github-actions-release-flow.md",
	"docs/release/local-delivery.md",
	"docs/release/local-release-transaction.md",
	"docs/release/recovery-model.md",
	"docs/release/state.md",
	"docs/release/tag-strategy.md",
	"docs/release/unit-selection.md",
	"docs/release/version-materialization.md",
}

func TestDocumentationHasOneCurrentOwnerPerMajorTopic(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range mergedOrRemovedDocumentation {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("merged or removed documentation remains active at %s", path)
		}
	}

	assertUniqueDocumentationClaim(t, root, "canonical Core CLI command reference", "docs/cli-reference.md")
	assertUniqueDocumentationClaim(t, root, "canonical Release command reference", "docs/release/cli-reference.md")
	assertUniqueDocumentationClaim(t, root, "canonical Release documentation entry point", "docs/release/overview.md")
	assertUniqueDocumentationClaim(t, root, "Define the authoritative V2 files", "docs/release/configuration.md")
	assertUniqueDocumentationClaim(t, root, "This document owns the lifecycle concepts", "docs/release/lifecycle.md")
	assertUniqueDocumentationClaim(t, root, "Define workflow ownership", "docs/release/github-actions-delivery.md")
	assertUniqueDocumentationClaim(t, root, "Define durable execution evidence", "docs/release/journals-and-recovery.md")
}

func TestActiveProductDocumentsStatePurposeAndAudience(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range activeDocumentationPaths(t, root) {
		if path == "README.md" || path == "plugin/release/AGENTS.md" || path == "plugin/release/RULES.md" {
			continue
		}
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		for _, field := range []string{"> **Audience:**", "> **Purpose:**"} {
			if !strings.Contains(content, field) {
				t.Errorf("%s does not state %s", path, strings.Trim(field, "> *"))
			}
		}
	}
}

func TestActiveDocumentationContainsNoRoadmapOrProgressProse(t *testing.T) {
	root := repositoryRoot(t)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\broadmap\b`),
		regexp.MustCompile(`(?i)\bwork[ -]in[ -]progress\b`),
		regexp.MustCompile(`(?i)\bnext steps?\b`),
		regexp.MustCompile(`(?i)\bimplemented now\b`),
		regexp.MustCompile(`(?i)\bnot (?:yet|currently) (?:implemented|available|supported)\b`),
		regexp.MustCompile(`(?i)\b(?:future|later) (?:work|product|capabilit(?:y|ies)|integration|design|implementation|release|removal|migration)\b`),
		regexp.MustCompile(`(?i)\b(?:will eventually|will be implemented|planned for)\b`),
		regexp.MustCompile(`(?i)\b(?:completed|post-)refactor\b`),
		regexp.MustCompile(`(?i)\bmilestone [0-9]+\b`),
		regexp.MustCompile(`(?i)^#{1,6} .*\b(?:future|roadmap|progress|next steps?)\b`),
	}

	for _, path := range activeDocumentationPaths(t, root) {
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		for lineNumber, line := range strings.Split(content, "\n") {
			for _, pattern := range patterns {
				if pattern.MatchString(line) {
					if allowedRoadmapOrProgressLine(path, line) {
						continue
					}
					t.Errorf("%s:%d contains active roadmap/progress wording %q", path, lineNumber+1, strings.TrimSpace(line))
				}
			}
		}
	}
}

func allowedRoadmapOrProgressLine(path, line string) bool {
	allowed := map[string]map[string]bool{
		"plugin/release/RULES.md": {
			"A completed/handoff-ready release must not be redispatched. Starting a later release is a separate intent calculated from current state.": true,
		},
	}
	return allowed[path][strings.TrimSpace(line)]
}

func TestActiveDocumentationContainsNoLocalPathsSecretsOrPinnedProductVersions(t *testing.T) {
	root := repositoryRoot(t)
	forbidden := []*regexp.Regexp{
		regexp.MustCompile(`/Users/[^\s)` + "`" + `]+`),
		regexp.MustCompile(`/home/[^\s)` + "`" + `]+`),
		regexp.MustCompile(`/private/tmp/[^\s)` + "`" + `]*`),
		regexp.MustCompile(`(?i)github_token\s*=`),
		regexp.MustCompile(`(?i)\b(?:ghp_|github_pat_)[A-Za-z0-9_]+`),
		regexp.MustCompile(`(?i)authorization:\s*bearer\s+\S+`),
		regexp.MustCompile("`v?[0-9]+\\.[0-9]+\\.[0-9]+`"),
	}
	for _, path := range activeDocumentationPaths(t, root) {
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		for _, pattern := range forbidden {
			if match := pattern.FindString(content); match != "" {
				if allowedActiveDocumentationValue(path, match) {
					continue
				}
				t.Errorf("%s contains local, credential-like, or pinned product value %q", path, match)
			}
		}
	}
}

func allowedActiveDocumentationValue(path, value string) bool {
	allowed := map[string]map[string]bool{
		"docs/release/cli-reference.md": {"`0.1.0`": true},
	}
	return allowed[path][value]
}

func TestCanonicalDocumentationStatesSafetyAndCompatibilityBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	required := map[string][]string{
		"docs/release/compatibility.md": {
			"V1 is supported compatibility, not the canonical architecture for new setup",
			"V2 is the canonical architecture for new repositories",
			"mixed V1 and V2 authority is rejected",
		},
		"docs/release/cli-reference.md": {
			"Doctor is read-only and never repairs",
			"Workflow Init is create-only",
			"Pipeline is read-only",
			"`--output-file` writes Plugin Index schema-v1 bytes",
		},
		"docs/installation.md": {
			"`--force` permits only a same-version reinstall",
			"same-directory sibling followed by atomic rename",
			"never invokes `sudo` automatically",
		},
		"docs/plugins/ui.md": {
			"advertises `hello`",
			"does not route a `hello` handler",
		},
	}
	for path, fragments := range required {
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		for _, fragment := range fragments {
			if !strings.Contains(content, fragment) {
				t.Errorf("%s is missing current contract %q", path, fragment)
			}
		}
	}
}

func assertUniqueDocumentationClaim(t *testing.T, root, claim, owner string) {
	t.Helper()
	var owners []string
	for _, path := range activeDocumentationPaths(t, root) {
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		if strings.Contains(content, claim) {
			owners = append(owners, path)
		}
	}
	sort.Strings(owners)
	if len(owners) != 1 || owners[0] != owner {
		t.Errorf("claim %q owners = %v, want [%s]", claim, owners, owner)
	}
}

func activeDocumentationPaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	for _, path := range trackedFiles(t, root) {
		if !strings.HasSuffix(path, ".md") || strings.HasPrefix(path, releaseHistoryDirectory+"/") {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}
