package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfReleaseReadmeIsConciseAndCurrent(t *testing.T) {
	root := repositoryRoot(t)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))
	section := selfReleaseReadmeSection(t, readme)

	if strings.Contains(readme, "### Release V2 Dogfood") {
		t.Error("README retains the oversized Release V2 dogfood heading")
	}
	if count := strings.Count(readme, "### How Neko CLI releases itself"); count != 1 {
		t.Errorf("current self-release heading occurs %d times, want exactly once", count)
	}
	if words := len(strings.Fields(section)); words < 180 || words > 350 {
		t.Errorf("self-release section contains %d words, want 180 through 350", words)
	}

	for _, required := range []string{
		"Neko CLI dogfoods Release V2 through three independently versioned release units.",
		"Each is configured with GoReleaser and GitHub Actions delivery.",
		"Their current versions are owned exclusively by `.neko/release.state.json`",
		"`neko release patch|minor|major` plans and materializes the selected unit",
		"Neko pushes the owned Git state",
		"dispatches the configured workflow with validated release identity",
		"The consumer-owned GitHub Actions workflow builds, creates the GitHub Release, and publishes the unit's artifacts.",
		"Dispatch is the handoff to that workflow, not completed publication.",
	} {
		if !strings.Contains(section, required) {
			t.Errorf("README self-release section is missing release-flow contract %q", required)
		}
	}

	for step := 1; step <= 4; step++ {
		marker := "\n" + string(rune('0'+step)) + ". "
		if count := strings.Count(section, marker); count != 1 {
			t.Errorf("README self-release flow step %d occurs %d times, want exactly once", step, count)
		}
	}
}

func TestSelfReleaseReadmeInspectionAndScaffoldingBoundaries(t *testing.T) {
	root := repositoryRoot(t)
	section := selfReleaseReadmeSection(t, readDocumentationFile(t, filepath.Join(root, "README.md")))

	for _, exact := range []string{
		"`neko release doctor` is read-only and validates the local V2 integration.",
		"It is offline and token-free by default; remote verification performs bounded GitHub GETs only when `--verify-remote` is explicitly requested.",
		"Doctor never repairs configuration or files.",
		"`neko release github-workflow-init` is create-only.",
		"It creates one missing starter workflow and accepts an identical existing workflow without rewriting it; differing customized content is never overwritten.",
		"Repository workflows remain intentionally consumer-owned after scaffolding.",
	} {
		if !strings.Contains(section, exact) {
			t.Errorf("README self-release section is missing exact boundary wording %q", exact)
		}
	}
}

func TestSelfReleaseReadmeUsesConfiguredTagsAndWorkflows(t *testing.T) {
	root := repositoryRoot(t)
	configuration, state := readSelfReleaseRepositoryFacts(t, root)
	section := selfReleaseReadmeSection(t, readDocumentationFile(t, filepath.Join(root, "README.md")))

	for _, unit := range configuration.Units {
		tagNamespace := "`" + unit.TagPrefix + "X.Y.Z`"
		if count := strings.Count(section, tagNamespace); count != 1 {
			t.Errorf("configured tag namespace %q occurs %d times, want exactly once", tagNamespace, count)
		}
		workflow := "`" + unit.Executor.Workflow + "`"
		if count := strings.Count(section, workflow); count != 1 {
			t.Errorf("configured workflow %q occurs %d times, want exactly once", workflow, count)
		}
		if version := state.Units[unit.ID].Version; version != "" && strings.Contains(section, version) {
			t.Errorf("README self-release section hard-codes current version %q", version)
		}
	}
}

func TestSelfReleaseReadmeLinksToCanonicalDetails(t *testing.T) {
	root := repositoryRoot(t)
	section := selfReleaseReadmeSection(t, readDocumentationFile(t, filepath.Join(root, "README.md")))
	requiredTargets := []string{
		"docs/release/cli-reference.md",
		"docs/release/migration-v1-to-v2.md",
		"docs/release/github-actions-golden-path.md",
		"docs/release/examples.md#normal-release-units-vs-neko-cli-plugin-units",
		"plugin/release/docs/architecture/current-state.md#historical-context",
	}
	for _, target := range requiredTargets {
		if !strings.Contains(section, "]("+target+")") {
			t.Errorf("README self-release section does not link to %s", target)
		}
	}
	if targets := markdownTargets(section); len(targets) != len(requiredTargets) {
		t.Errorf("README self-release section has %d links, want exactly %d focused links", len(targets), len(requiredTargets))
	}
}

func TestSelfReleaseReadmeOmitsRelocatedDetail(t *testing.T) {
	root := repositoryRoot(t)
	section := selfReleaseReadmeSection(t, readDocumentationFile(t, filepath.Join(root, "README.md")))
	for _, stale := range []string{
		"### Release V2 Dogfood",
		"the old `.plugin.release.neko.json` map has been removed",
		"`neko release init` creates V2",
		"`--kind release` is the default",
		"Runtime plugin discovery",
		"`/releases/latest`",
		"Use global `--describe`",
		"Static and complete read-only queries",
	} {
		if strings.Contains(section, stale) {
			t.Errorf("README self-release section retains relocated or stale detail %q", stale)
		}
	}
}
