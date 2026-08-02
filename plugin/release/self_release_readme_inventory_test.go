package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

type selfReleaseConfiguration struct {
	Units []selfReleaseUnit `json:"units"`
}

type selfReleaseUnit struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	TagPrefix string `json:"tagPrefix"`
	Plugin    *struct {
		Name        string `json:"name"`
		Manifest    string `json:"manifest"`
		AssetPrefix string `json:"assetPrefix"`
		BinaryName  string `json:"binaryName"`
	} `json:"plugin"`
	Executor struct {
		Type     string `json:"type"`
		Delivery string `json:"delivery"`
		Workflow string `json:"workflow"`
	} `json:"executor"`
}

type selfReleaseState struct {
	Units map[string]struct {
		Version string `json:"version"`
	} `json:"units"`
}

func TestSelfReleaseReadmeInventoryMatchesRepositoryConfiguration(t *testing.T) {
	root := repositoryRoot(t)
	configuration, state := readSelfReleaseRepositoryFacts(t, root)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))
	section := selfReleaseReadmeSection(t, readme)

	if len(configuration.Units) != 3 {
		t.Fatalf("configured self-release units = %d, want three", len(configuration.Units))
	}
	if len(state.Units) != len(configuration.Units) {
		t.Fatalf("state units = %d, configuration units = %d", len(state.Units), len(configuration.Units))
	}

	workflowOwners := make(map[string]string, len(configuration.Units))
	for _, unit := range configuration.Units {
		if count := strings.Count(section, "`"+unit.ID+"`"); count != 1 {
			t.Errorf("README self-release section names configured unit %q %d times, want exactly once", unit.ID, count)
		}
		stateUnit, ok := state.Units[unit.ID]
		if !ok {
			t.Errorf("configured unit %q has no state entry", unit.ID)
			continue
		}
		if stateUnit.Version == "" {
			t.Errorf("configured unit %q has an empty state version", unit.ID)
		}
		if strings.Contains(section, stateUnit.Version) {
			t.Errorf("README self-release section hard-codes current version %q for unit %q", stateUnit.Version, unit.ID)
		}

		tagExample := regexp.MustCompile("`" + regexp.QuoteMeta(unit.TagPrefix) + `(?:X\.Y\.Z|[0-9]+\.[0-9]+\.[0-9]+)` + "`")
		if !tagExample.MatchString(section) {
			t.Errorf("README self-release section has no tag namespace example for configured prefix %q", unit.TagPrefix)
		}
		if unit.Executor.Type != "goreleaser" || unit.Executor.Delivery != "github-actions" {
			t.Errorf("unit %q executor = %q/%q, want goreleaser/github-actions", unit.ID, unit.Executor.Type, unit.Executor.Delivery)
		}
		if owner, exists := workflowOwners[unit.Executor.Workflow]; exists {
			t.Errorf("workflow %q is shared by %q and %q, want independent workflows", unit.Executor.Workflow, owner, unit.ID)
		}
		workflowOwners[unit.Executor.Workflow] = unit.ID
		workflowInfo, err := os.Stat(filepath.Join(root, filepath.FromSlash(unit.Executor.Workflow)))
		if err != nil {
			t.Errorf("configured workflow for unit %q does not exist: %v", unit.ID, err)
		} else if !workflowInfo.Mode().IsRegular() {
			t.Errorf("configured workflow for unit %q is not a regular file", unit.ID)
		}

		if unit.Kind == "plugin" {
			if unit.Plugin == nil || unit.Plugin.Name == "" || unit.Plugin.Manifest == "" ||
				unit.Plugin.AssetPrefix == "" || unit.Plugin.BinaryName == "" {
				t.Errorf("plugin unit %q has incomplete plugin metadata", unit.ID)
			}
		} else if unit.Kind != "" || unit.Plugin != nil {
			t.Errorf("normal unit %q has unexpected kind or plugin metadata", unit.ID)
		}
	}

	for _, required := range []string{
		"`.neko/release.state.json`",
		"](docs/release/cli-reference.md)",
		"](docs/release/github-actions-golden-path.md)",
	} {
		if !strings.Contains(section, required) {
			t.Errorf("README self-release section is missing %q", required)
		}
	}
}

func TestDetailedSelfReleaseClaimsRemainOwnedByCanonicalDocuments(t *testing.T) {
	root := repositoryRoot(t)
	requiredByDocument := map[string][]string{
		"docs/release/cli-reference.md": {
			"## Release V1 versus Release V2",
			"`unit-add` appends one unit to existing V2 config/state.",
			"The command is create-only:",
			"### Pipeline inspection",
			"## Resume",
		},
		"docs/release/examples.md": {
			"## Normal Release Units Vs Neko CLI Plugin Units",
			"`release.state.json` is authoritative version state.",
			"Plugin metadata lives on the V2 unit.",
			"public registry index.",
		},
		"docs/release/github-actions-golden-path.md": {
			"## Release plan and dry-run",
			"## Evidence and recovery",
			"An accepted dispatch is a successful Neko-to-GitHub handoff",
		},
		"docs/release/github-actions-delivery.md": {
			"Neko CLI and the Release Plugin own release policy",
			"Consumer repositories own",
		},
		"docs/installation.md": {
			"filters for stable CLI tags matching `vX.Y.Z`",
			"Plugin discovery, install, and update use the published `plugin-index.json`",
		},
		"docs/plugins/release.md": {
			"`--describe` adds complete safe human facts",
			"phases where available and is likewise a no-op on static commands.",
		},
		"plugin/release/docs/architecture/current-state.md": {
			"# Release Plugin Current Architecture",
			"#### V2 GitHub Actions execution",
		},
		"plugin/release/docs/history/README.md": {
			"This folder preserves completed or superseded Release planning",
			"not the active",
			"product or architecture source.",
		},
	}

	for path, fragments := range requiredByDocument {
		content := readDocumentationFile(t, filepath.Join(root, filepath.FromSlash(path)))
		for _, fragment := range fragments {
			if !strings.Contains(content, fragment) {
				t.Errorf("%s no longer owns detailed self-release claim %q", path, fragment)
			}
		}
	}
}

func readSelfReleaseRepositoryFacts(t *testing.T, root string) (selfReleaseConfiguration, selfReleaseState) {
	t.Helper()
	var configuration selfReleaseConfiguration
	configurationBytes, err := os.ReadFile(filepath.Join(root, ".neko", "release.config.json"))
	if err != nil {
		t.Fatalf("read self-release configuration: %v", err)
	}
	if decodeErr := json.Unmarshal(configurationBytes, &configuration); decodeErr != nil {
		t.Fatalf("decode self-release configuration: %v", decodeErr)
	}

	var state selfReleaseState
	stateBytes, err := os.ReadFile(filepath.Join(root, ".neko", "release.state.json"))
	if err != nil {
		t.Fatalf("read self-release state: %v", err)
	}
	if decodeErr := json.Unmarshal(stateBytes, &state); decodeErr != nil {
		t.Fatalf("decode self-release state: %v", decodeErr)
	}
	return configuration, state
}

func selfReleaseReadmeSection(t *testing.T, readme string) string {
	t.Helper()
	start := strings.Index(readme, "### Release V2 Dogfood\n")
	if start < 0 {
		start = strings.Index(readme, "### How Neko CLI releases itself\n")
	}
	if start < 0 {
		t.Fatal("README self-release section is missing")
	}
	remainder := readme[start:]
	next := strings.Index(remainder[1:], "\n### ")
	if next < 0 {
		return remainder
	}
	return remainder[:next+1]
}
