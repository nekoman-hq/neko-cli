package main

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	readmeBadgeStart = "<!-- hero-badges:start -->"
	readmeBadgeEnd   = "<!-- hero-badges:end -->"
)

type readmeManifest struct {
	Commands []struct {
		Name  string `json:"name"`
		Flags []struct {
			Name string `json:"name"`
		} `json:"flags"`
	} `json:"commands"`
}

func TestReadmeHeroUsesAdaptiveShieldcnAssets(t *testing.T) {
	root := repositoryRoot(t)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))

	for _, required := range []string{
		"https://shieldcn.dev/header/graph.svg",
		"mode=light",
		"mode=dark",
		"title=Neko%20CLI",
		"logo=go",
		"<h1 align=\"center\">Neko CLI</h1>",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README hero is missing %q", required)
		}
	}
	if count := strings.Count(readme, "https://shieldcn.dev/header/"); count != 2 {
		t.Errorf("adaptive README header URLs = %d, want light and dark", count)
	}
	if strings.Contains(readme, "img.shields.io") {
		t.Error("README retains a Shields.io asset")
	}

	heroEnd := strings.Index(readme, "## ")
	if heroEnd < 0 {
		t.Fatal("README has no section after its hero")
	}
	hero := readme[:heroEnd]
	altPattern := regexp.MustCompile(`<img alt="([^"]+)"`)
	alts := altPattern.FindAllStringSubmatch(hero, -1)
	if len(alts) != 5 {
		t.Fatalf("hero image alt texts = %d, want one header and four badges", len(alts))
	}
	for _, match := range alts {
		alt := strings.TrimSpace(match[1])
		if len(alt) < 8 || alt == "badge" || alt == "image" || alt == "logo" {
			t.Errorf("README hero has unhelpful alt text %q", alt)
		}
	}
}

func TestReadmeShieldcnBadgeContract(t *testing.T) {
	root := repositoryRoot(t)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))
	badges := readmeMarkedSection(t, readme, readmeBadgeStart, readmeBadgeEnd)

	if count := strings.Count(badges, "<picture>"); count < 3 || count > 6 {
		t.Errorf("README hero badge count = %d, want three through six", count)
	}
	if count := strings.Count(badges, "<picture>"); count != 4 {
		t.Errorf("README hero badge count = %d, want the four selected high-signal badges", count)
	}
	for _, required := range []string{
		"/github/dt/nekoman-hq/neko-cli.svg",
		"/github/ci/nekoman-hq/neko-cli.svg?workflow=go-lint.yml&amp;branch=main",
		"/badge/License-Personal--noncommercial-64748b.svg",
		"/badge/Go-1.24.4-00ADD8.svg",
		"variant=secondary",
		"variant=branded",
	} {
		if !strings.Contains(badges, required) {
			t.Errorf("README badge row is missing %q", required)
		}
	}
	if strings.Contains(badges, "split=true") || strings.Contains(badges, ".png") {
		t.Error("README badge row must use single-surface SVG badges")
	}
	if strings.Count(badges, "variant=branded") != 2 {
		t.Error("only the light and dark forms of one badge may use the branded variant")
	}

	for _, destination := range []string{
		"https://github.com/nekoman-hq/neko-cli/releases",
		"https://github.com/nekoman-hq/neko-cli/actions/workflows/go-lint.yml",
		"https://github.com/nekoman-hq/neko-cli/blob/main/LICENSE",
		"https://github.com/nekoman-hq/neko-cli/blob/main/go.mod",
	} {
		if strings.Count(badges, `href="`+destination+`"`) != 1 {
			t.Errorf("README badge destination %q must occur exactly once", destination)
		}
	}
}

func TestReadmeBadgeMetadataMatchesRepository(t *testing.T) {
	root := repositoryRoot(t)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))
	badges := html.UnescapeString(readmeMarkedSection(t, readme, readmeBadgeStart, readmeBadgeEnd))

	module := readDocumentationFile(t, filepath.Join(root, "go.mod"))
	if !strings.HasPrefix(module, "module github.com/nekoman-hq/neko-cli\n") {
		t.Fatalf("unexpected module identity in go.mod")
	}
	goVersion := regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)$`).FindStringSubmatch(module)
	if len(goVersion) != 2 {
		t.Fatal("go.mod has no exact Go requirement")
	}
	if !strings.Contains(badges, "/badge/Go-"+goVersion[1]+"-00ADD8.svg") {
		t.Errorf("README Go badge does not use go.mod requirement %s", goVersion[1])
	}

	workflow := readDocumentationFile(t, filepath.Join(root, ".github/workflows/go-lint.yml"))
	workflowName := regexp.MustCompile(`(?m)^name: ([^\n]+)$`).FindStringSubmatch(workflow)
	if len(workflowName) != 2 || workflowName[1] != "golangci-lint" {
		t.Fatalf("lint workflow name = %v, want golangci-lint", workflowName)
	}
	if !strings.Contains(badges, "workflow=go-lint.yml&branch=main") {
		t.Error("README CI badge does not select the real workflow and default branch")
	}
	if !strings.Contains(badges, "/badge/License-Personal--noncommercial-64748b.svg") {
		t.Error("README license badge must state the repository's custom license instead of relying on GitHub classification")
	}
}

func TestReadmeExamplesUseRegisteredCommandsAndFlags(t *testing.T) {
	root := repositoryRoot(t)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))
	wantCommands := map[string]bool{
		"neko version":                                   true,
		"neko plugin available":                          true,
		"neko plugin install release":                    true,
		"neko release --help":                            true,
		"neko release doctor":                            true,
		"neko release init-options":                      true,
		"neko release units":                             true,
		"neko release pipeline":                          true,
		"neko release plan --change patch":               true,
		"neko release plan --change patch --output json": true,
		"neko release plugin-index --output-file build/plugin-index.json": true,
	}
	commandPattern := regexp.MustCompile(`(?m)^neko(?: [^\n]+)?$`)
	commands := commandPattern.FindAllString(readme, -1)
	seen := make(map[string]bool, len(commands))
	for _, command := range commands {
		if !wantCommands[command] {
			t.Errorf("README uses unregistered or unreviewed command example %q", command)
		}
		seen[command] = true
	}
	for command := range wantCommands {
		if !seen[command] {
			t.Errorf("README is missing verified command example %q", command)
		}
	}

	manifest := readReadmeManifest(t, filepath.Join(root, "plugin/release/manifest.json"))
	flagsByCommand := make(map[string]map[string]bool, len(manifest.Commands))
	for _, command := range manifest.Commands {
		flagsByCommand[command.Name] = make(map[string]bool, len(command.Flags))
		for _, flag := range command.Flags {
			flagsByCommand[command.Name]["--"+flag.Name] = true
		}
	}
	globalFlags := map[string]bool{"--help": true, "--output": true}
	for _, command := range commands {
		fields := strings.Fields(command)
		if len(fields) < 3 || fields[1] != "release" {
			continue
		}
		commandName := fields[2]
		if strings.HasPrefix(commandName, "--") {
			commandName = ""
		}
		for _, field := range fields[2:] {
			if !strings.HasPrefix(field, "--") {
				continue
			}
			if !globalFlags[field] && !flagsByCommand[commandName][field] {
				t.Errorf("README command %q uses unregistered flag %q", command, field)
			}
		}
	}
}

func TestReadmeStatesCurrentSafetyContracts(t *testing.T) {
	root := repositoryRoot(t)
	readme := normalizedReadmeText(readDocumentationFile(t, filepath.Join(root, "README.md")))
	for _, required := range []string{
		"Doctor is read-only and never repairs repository configuration or workflows.",
		"Workflow Init is create-only and never overwrites a differing customized workflow.",
		"Pipeline is a read-only projection; it does not execute, retry, or resume lifecycle stages.",
		"Plugin Index persistence uses `--output-file`",
		"Core `--output` selects a response format",
		"`neko update --force` is a same-version reinstall only.",
		"It does not bypass permissions, package-manager ownership, integrity checks, platform checks, or downgrade protection.",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("README is missing current safety contract %q", required)
		}
	}
}

func normalizedReadmeText(content string) string {
	return strings.Join(strings.Fields(content), " ")
}

func TestReadmeContainsNoStateVersionsRoadmapPathsSecretsOrPlaceholders(t *testing.T) {
	root := repositoryRoot(t)
	readme := readDocumentationFile(t, filepath.Join(root, "README.md"))
	_, state := readSelfReleaseRepositoryFacts(t, root)
	for unitID, unit := range state.Units {
		if unit.Version != "" && strings.Contains(readme, unit.Version) {
			t.Errorf("README hard-codes current version %q for %s", unit.Version, unitID)
		}
	}

	for _, pattern := range []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:recently|upcoming|planned|future work|next phase|coming soon|will support|not yet implemented|after the refactor|post-refactor)\b`),
		regexp.MustCompile(`/Users/|/home/|/private/tmp/`),
		regexp.MustCompile(`(?i)github_token\s*=|gh_token\s*=|authorization:\s*bearer|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+`),
		regexp.MustCompile(`(?i)\bTODO\b|\bTBD\b|\bPLACEHOLDER\b|example\.com|your-org|your-repo`),
	} {
		if match := pattern.FindString(readme); match != "" {
			t.Errorf("README contains forbidden current-state content %q", match)
		}
	}
}

func readmeMarkedSection(t *testing.T, content, startMarker, endMarker string) string {
	t.Helper()
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("README markers %q and %q are missing or out of order", startMarker, endMarker)
	}
	return content[start+len(startMarker) : end]
}

func readReadmeManifest(t *testing.T, path string) readmeManifest {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest readmeManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}
