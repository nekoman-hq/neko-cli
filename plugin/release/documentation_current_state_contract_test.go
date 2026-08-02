package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var finalActiveDocumentation = map[string]string{
	"README.md":                                                   "repository entry point",
	"docs/ai_context.md":                                          "AI bootstrap context",
	"docs/cli-reference.md":                                       "Core CLI command reference",
	"docs/installation.md":                                        "installation and self-update",
	"docs/package-architecture.md":                                "repository package architecture",
	"docs/plugin-development.md":                                  "plugin authoring",
	"docs/plugins/release.md":                                     "Release Plugin entry point",
	"docs/plugins/ui.md":                                          "UI Plugin entry point",
	"docs/release/cli-reference.md":                               "Release command reference",
	"docs/release/compatibility.md":                               "V1/V2 compatibility",
	"docs/release/configuration.md":                               "Release configuration, state, units, and tags",
	"docs/release/examples.md":                                    "Release examples",
	"docs/release/github-actions-delivery.md":                     "GitHub Actions ownership and dispatch",
	"docs/release/github-actions-golden-path.md":                  "GitHub Actions operator guide",
	"docs/release/integration-doctor-remote-verification.md":      "opt-in remote Doctor verification",
	"docs/release/journals-and-recovery.md":                       "execution evidence and recovery",
	"docs/release/lifecycle.md":                                   "Release lifecycle and delivery boundary",
	"docs/release/migration-v1-to-v2.md":                          "V1-to-V2 migration",
	"docs/release/overview.md":                                    "Release product overview",
	"plugin/release/AGENTS.md":                                    "Release contributor instructions",
	"plugin/release/RULES.md":                                     "Release contributor rules",
	"plugin/release/docs/architecture/architecture-decisions.md":  "Release architecture constraints",
	"plugin/release/docs/architecture/compatibility-notes.md":     "Release package compatibility",
	"plugin/release/docs/architecture/current-state.md":           "Release implementation architecture",
	"plugin/release/docs/architecture/maintainability-policy.md":  "Release maintenance policy",
	"plugin/release/docs/architecture/package-ownership.md":       "Release package ownership",
	"plugin/release/docs/architecture/v1-compatibility-policy.md": "V1 compatibility policy",
}

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
	for path, topic := range finalActiveDocumentation {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("canonical owner for %s is missing at %s: %v", topic, path, err)
			continue
		}
		if !info.Mode().IsRegular() {
			t.Errorf("canonical owner for %s is not a regular file: %s", topic, path)
		}
	}
	for _, path := range mergedOrRemovedDocumentation {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("merged or removed documentation remains active at %s", path)
		}
	}

	assertUniqueDocumentationClaim(t, root, "canonical Core CLI command reference", "docs/cli-reference.md")
	assertUniqueDocumentationClaim(t, root, "canonical Release command reference", "docs/release/cli-reference.md")
	assertUniqueDocumentationClaim(t, root, "canonical Release documentation entry point", "docs/release/overview.md")
}

func TestActiveProductDocumentsStatePurposeAndAudience(t *testing.T) {
	root := repositoryRoot(t)
	for path := range finalActiveDocumentation {
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

	allowedPolicyDocuments := map[string]bool{
		"plugin/release/AGENTS.md": true,
		"plugin/release/RULES.md":  true,
	}
	for path := range finalActiveDocumentation {
		if allowedPolicyDocuments[path] {
			continue
		}
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		for lineNumber, line := range strings.Split(content, "\n") {
			for _, pattern := range patterns {
				if pattern.MatchString(line) {
					t.Errorf("%s:%d contains active roadmap/progress wording %q", path, lineNumber+1, strings.TrimSpace(line))
				}
			}
		}
	}
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
	for path := range finalActiveDocumentation {
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
		"plugin/release/RULES.md":       {"/private/tmp/neko-cli-go-build": true},
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
	for path := range finalActiveDocumentation {
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
