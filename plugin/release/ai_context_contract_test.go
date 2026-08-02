package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAIContextIsCompactCurrentBootstrap(t *testing.T) {
	context := readAIContext(t)
	if words := len(strings.Fields(context)); words < 1500 || words > 2800 {
		t.Errorf("AI context contains %d words, want 1500 through 2800", words)
	}

	for _, heading := range []string{
		"# AI Context for Neko CLI",
		"## Purpose and authority",
		"## Repository and Core CLI",
		"## Plugin transport and exit ownership",
		"## Release V1 compatibility",
		"## Release V2 architecture",
		"## Inspection and safety boundaries",
		"## Release lifecycle and recovery",
		"## Architecture constraints",
		"## Self-update and installation",
		"## Self-release of this repository",
		"## Documentation navigation",
		"## Active versus completed work",
		"## Do not assume",
	} {
		if count := strings.Count(context, heading); count != 1 {
			t.Errorf("AI context heading %q occurs %d times, want once", heading, count)
		}
	}
}

func TestAIContextLinksCanonicalContractsAndCoversReleasePaths(t *testing.T) {
	context := readAIContext(t)
	for _, target := range []string{
		"cli-reference.md",
		"release/cli-reference.md",
		"release/compatibility.md",
		"release/migration-v1-to-v2.md",
		"installation.md",
		"../plugin/release/docs/architecture/current-state.md",
		"../plugin/release/docs/architecture/package-ownership.md",
		"../plugin/release/docs/architecture/architecture-decisions.md",
		"../plugin/release/docs/history/README.md",
	} {
		if !strings.Contains(context, "]("+target) {
			t.Errorf("AI context does not link to %s", target)
		}
	}

	if !strings.Contains(context, "`neko release`") {
		t.Error("AI context omits the Release manifest overview path")
	}
	for name := range loadManifestCommands(t) {
		path := "`neko release " + name + "`"
		if !strings.Contains(context, path) {
			t.Errorf("AI context capability groups omit %s", path)
		}
	}
}

func TestAIContextSeparatesV1AndV2Authority(t *testing.T) {
	context := normalizedAIContext(t)
	for _, required := range []string{
		"V1 is supported compatibility, not the canonical architecture",
		"`.release.neko.json` is the V1 authority",
		"one virtual default unit",
		"GoReleaser, JReleaser, and release-it",
		"V2 is the canonical architecture for new repositories",
		"`.neko/release.config.json` owns declared configuration",
		"`.neko/release.state.json` owns current unit versions",
		"V1 and V2 cannot remain active as competing authorities",
		"`neko release migrate` is the supported transition",
		"dispatch is a handoff, not publication completion",
	} {
		if !strings.Contains(context, required) {
			t.Errorf("AI context is missing V1/V2 contract %q", required)
		}
	}
}

func TestAIContextStatesInspectionAndMutationBoundaries(t *testing.T) {
	context := normalizedAIContext(t)
	for _, required := range []string{
		"Doctor is strictly read-only",
		"offline and token-free by default",
		"explicit `--verify-remote`",
		"bounded GET-only verification",
		"never repairs configuration, files, or workflows",
		"Units is a read-only unit inventory",
		"Pipeline is a read-only local projection",
		"not a lifecycle engine or state machine",
		"Plan is read-only release planning",
		"Evidence is read-only inspection",
		"Evidence Archive is a separate, explicit guarded local mutation",
		"Workflow Init is create-only",
		"never overwrites differing customized content",
		"Resume continues one existing unresolved execution",
		"does not create a new release",
		"`--describe` and `--verbose` never add network, token, or mutation reachability",
	} {
		if !strings.Contains(context, required) {
			t.Errorf("AI context is missing safety boundary %q", required)
		}
	}
}

func TestAIContextStatesOutputAndExitOwnership(t *testing.T) {
	context := normalizedAIContext(t)
	for _, required := range []string{
		"global `--describe`, `--verbose`, `--output`, and `--github-output-file` flags",
		"`table`, `wide`, `json`, or `github`",
		"`--output` selects the Core response format",
		"`--output-file` persists Plugin Index schema-v1 bytes",
		"public `plugin.Response` envelope",
		"presentation-only metadata excluded",
		"Plugin Index is a deliberate raw-output exception",
		"valid decoded `plugin.Response` owns the final process exit",
		"explicit exits from `0` through `125` propagate exactly",
		"Release commands assign only `0` and `1`",
		"omitted exits are temporary legacy compatibility",
		"transport and rendering failures are Core-owned",
		"renders exactly once",
		"Pipeline blocked → `0`",
		"Pipeline invalid evidence → `1`",
		"Doctor warning → `0`",
		"Doctor not ready → `1`",
		"Resume unsafe dry-run → `0`",
		"Resume no journal → `1`",
	} {
		if !strings.Contains(context, required) {
			t.Errorf("AI context is missing output/exit contract %q", required)
		}
	}
}

func TestAIContextPreservesArchitectureConstraints(t *testing.T) {
	context := normalizedAIContext(t)
	for _, required := range []string{
		"`cmd` | CLI composition, global flags, plugin routing, rendering call, final process status",
		"`pkg/plugin` | neutral request, response, log, and explicit-exit transport contracts",
		"`pkg/dispatcher` | subprocess execution and response decoding",
		"`pkg/presentation` | domain-neutral presentation declarations",
		"`pkg/renderer` | responsive human, JSON, and GitHub rendering",
		"`internal/terminal` | terminal width, TTY, color, and display capabilities",
		"`plugin/release/pkg/release` | authoritative lifecycle, Git, journals, Resume, and recovery",
		"generic lifecycle framework",
		"second state machine",
		"stage registry",
		"command-handler chaining",
		"provider hierarchy",
		"dependency bag or service locator",
		"workflow DSL",
		"second renderer",
		"command-name switches in Core",
		"domain-status interpretation in Core",
		"Terminal dependencies stay outside domain and application logic",
	} {
		if !strings.Contains(context, required) {
			t.Errorf("AI context is missing architecture constraint %q", required)
		}
	}
}

func TestAIContextStatesSelfUpdateContract(t *testing.T) {
	context := normalizedAIContext(t)
	for _, required := range []string{
		"ordinary-user installer default is `$HOME/.local/bin`",
		"`NEKO_INSTALL_DIR` wins",
		"deliberate root execution may default to `/usr/local/bin`",
		"never invokes `sudo` automatically",
		"`neko update --force` means same-version reinstall only",
		"does not bypass permissions, integrity checks, package-manager ownership, or downgrade protection",
		"Homebrew-owned installations are refused",
		"checksum verification and archive validation are mandatory",
		"same-directory sibling followed by atomic rename",
		"Dry-run does not download the archive or replace the executable",
	} {
		if !strings.Contains(context, required) {
			t.Errorf("AI context is missing self-update contract %q", required)
		}
	}
}

func TestAIContextSelfReleaseFactsDeriveFromConfiguration(t *testing.T) {
	root := repositoryRoot(t)
	context := readAIContext(t)
	configuration, state := readSelfReleaseRepositoryFacts(t, root)
	for _, unit := range configuration.Units {
		if !strings.Contains(context, "`"+unit.ID+"`") {
			t.Errorf("AI context omits configured self-release unit %q", unit.ID)
		}
		if !strings.Contains(context, "`"+unit.TagPrefix+"X.Y.Z`") {
			t.Errorf("AI context omits tag namespace for configured unit %q", unit.ID)
		}
		if !strings.Contains(context, "`"+unit.Executor.Workflow+"`") {
			t.Errorf("AI context omits workflow for configured unit %q", unit.ID)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(unit.Executor.Workflow))); err != nil {
			t.Errorf("configured workflow for unit %q is missing: %v", unit.ID, err)
		}
		if version := state.Units[unit.ID].Version; version != "" && strings.Contains(context, version) {
			t.Errorf("AI context hard-codes current version %q for unit %q", version, unit.ID)
		}
	}
}

func TestAIContextOmitsStaleHistoryAndSensitiveValues(t *testing.T) {
	context := readAIContext(t)
	for _, stale := range []string{
		"`.{plugin}.neko.json`",
		"Future deploy plugin",
		"## Files to Ignore",
		"Version:   \"1.0.0\"",
		"\"table\", \"json\", or \"text\"",
		"`--output json` - Raw JSON",
		"plugin/release/docs/architecture/refactor-plan.md",
		"plugin/release/docs/architecture/refactor-history.md",
		"plugin/release/docs/architecture/post-refactor-review.md",
		"plugin/release/docs/architecture/post-refactor-roadmap.md",
		"plugin/release/docs/architecture/architecture-evolution.md",
	} {
		if strings.Contains(context, stale) {
			t.Errorf("AI context retains stale content %q", stale)
		}
	}

	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`/Users/[^\s)` + "`" + `]+`),
		regexp.MustCompile(`/home/[^\s)` + "`" + `]+`),
		regexp.MustCompile(`(?i)github_token\s*=`),
		regexp.MustCompile(`(?i)\b(?:ghp_|github_pat_)[A-Za-z0-9_]+`),
		regexp.MustCompile(`(?i)authorization:\s*bearer\s+\S+`),
	} {
		if match := pattern.FindString(context); match != "" {
			t.Errorf("AI context contains local or credential-like value %q", match)
		}
	}
}

func TestAIContextSeparatesCurrentWorkFromHistory(t *testing.T) {
	context := normalizedAIContext(t)
	for _, required := range []string{
		"numbered history for rationale only",
		"numbered history records completed or superseded work",
		"history is not implementation guidance",
		"not an implied roadmap",
		"V2 local execution",
		"public standalone dispatch or retry commands",
		"durable workflow-run or publication-completion state are absent",
	} {
		if !strings.Contains(context, required) {
			t.Errorf("AI context is missing current/history distinction %q", required)
		}
	}
	if strings.Contains(strings.ToLower(context), "milestone") {
		t.Error("AI context presents milestone language as current bootstrap content")
	}
}

func readAIContext(t *testing.T) string {
	t.Helper()
	return readDocumentationFile(t, filepath.Join(repositoryRoot(t), "docs", "ai_context.md"))
}

func normalizedAIContext(t *testing.T) string {
	t.Helper()
	return strings.Join(strings.Fields(readAIContext(t)), " ")
}
