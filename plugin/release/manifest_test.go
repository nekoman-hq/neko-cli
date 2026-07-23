package main

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type manifestCommand struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Outputs     []string `json:"outputs"`
	Flags       []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	} `json:"flags"`
}

func TestManifestMatchesPublicReleaseContract(t *testing.T) {
	commands := loadManifestCommands(t)
	expected := map[string]map[string]string{
		"init": {
			"unit":                "string",
			"display-name":        "string",
			"version":             "string",
			"executor":            "string",
			"delivery":            "string",
			"workflow":            "string",
			"tag-prefix":          "string",
			"working-directory":   "string",
			"paths":               "string",
			"kind":                "string",
			"plugin-name":         "string",
			"plugin-manifest":     "string",
			"plugin-asset-prefix": "string",
			"plugin-binary-name":  "string",
			"force":               "bool",
		},
		"init-options": {},
		"unit-add": {
			"unit":                "string",
			"display-name":        "string",
			"version":             "string",
			"executor":            "string",
			"delivery":            "string",
			"workflow":            "string",
			"tag-prefix":          "string",
			"working-directory":   "string",
			"paths":               "string",
			"kind":                "string",
			"plugin-name":         "string",
			"plugin-manifest":     "string",
			"plugin-asset-prefix": "string",
			"plugin-binary-name":  "string",
		},
		"patch": {
			"dry-run": "bool",
			"unit":    "string",
		},
		"minor": {
			"dry-run": "bool",
			"unit":    "string",
		},
		"major": {
			"dry-run": "bool",
			"unit":    "string",
		},
		"plan": {
			"change": "string",
			"unit":   "string",
		},
		"doctor": {
			"unit":          "string",
			"verify-remote": "bool",
		},
		"units": {},
		"pipeline": {
			"unit":          "string",
			"verify-remote": "bool",
		},
		"ci-validate-context": {
			"unit":        "string",
			"version":     "string",
			"tag":         "string",
			"release-sha": "string",
		},
		"github-workflow-init": {
			"unit":    "string",
			"path":    "string",
			"dry-run": "bool",
		},
		"history": {
			"unit": "string",
		},
		"contributors": {
			"unit": "string",
		},
		"validate": {
			"show": "bool",
			"unit": "string",
		},
		"migrate": {
			"dry-run": "bool",
		},
		"resume": {
			"unit":    "string",
			"dry-run": "bool",
		},
		"evidence": {
			"family":   "string",
			"unit":     "string",
			"identity": "string",
		},
		"evidence-archive": {
			"family":          "string",
			"identity":        "string",
			"digest-sha256":   "string",
			"confirm-archive": "bool",
		},
		"plugin-index": {
			"output":     "string",
			"check":      "bool",
			"pretty":     "bool",
			"repository": "string",
		},
	}

	if len(commands) != len(expected) {
		t.Fatalf("manifest command count drifted: got %d want %d", len(commands), len(expected))
	}
	for name, expectedFlags := range expected {
		command, ok := commands[name]
		if !ok {
			t.Fatalf("manifest command %q missing", name)
		}
		if strings.TrimSpace(command.Description) == "" {
			t.Fatalf("manifest command %q has empty description", name)
		}
		actualFlags := map[string]string{}
		for _, flag := range command.Flags {
			actualFlags[flag.Name] = flag.Type
		}
		if !stringMapEqual(actualFlags, expectedFlags) {
			t.Fatalf("manifest flags for %s drifted: got %#v want %#v", name, actualFlags, expectedFlags)
		}
	}
	for _, forbidden := range []string{"dispatch", "retry"} {
		if _, ok := commands[forbidden]; ok {
			t.Fatalf("manifest must not expose unsupported command %q", forbidden)
		}
	}
}

func TestReleaseContextValidationManifestContract(t *testing.T) {
	command, present := loadManifestCommands(t)["ci-validate-context"]
	if !present {
		t.Fatal("ci-validate-context command is missing")
	}
	if !reflect.DeepEqual(command.Outputs, []string{"table", "json", "github"}) {
		t.Fatalf("outputs = %#v", command.Outputs)
	}
	wantOrder := []string{"unit", "version", "tag", "release-sha"}
	gotOrder := make([]string, 0, len(command.Flags))
	for _, flag := range command.Flags {
		gotOrder = append(gotOrder, flag.Name)
		if !flag.Required || flag.Type != "string" {
			t.Fatalf("flag %s = %#v, want required string", flag.Name, flag)
		}
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("flag order = %#v, want %#v", gotOrder, wantOrder)
	}
}

func TestGitHubWorkflowScaffoldManifestContract(t *testing.T) {
	command, present := loadManifestCommands(t)["github-workflow-init"]
	if !present {
		t.Fatal("github-workflow-init command is missing")
	}
	if !reflect.DeepEqual(command.Outputs, []string{"table", "json"}) {
		t.Fatalf("outputs = %#v", command.Outputs)
	}
	wantFlags := []string{"unit", "path", "dry-run"}
	gotFlags := make([]string, 0, len(command.Flags))
	for _, flag := range command.Flags {
		gotFlags = append(gotFlags, flag.Name)
		if flag.Required {
			t.Fatalf("workflow scaffold flag %s must be optional", flag.Name)
		}
	}
	if !reflect.DeepEqual(gotFlags, wantFlags) {
		t.Fatalf("flag order = %#v, want %#v", gotFlags, wantFlags)
	}
	for _, forbidden := range []string{"provider", "force", "update", "consumer-command"} {
		if _, present := flagDescriptions(command)[forbidden]; present {
			t.Fatalf("workflow scaffold manifest exposes unsupported flag %q", forbidden)
		}
	}
}

func TestSetupAndMigrationManifestOutputContracts(t *testing.T) {
	commands := loadManifestCommands(t)
	for _, name := range []string{"init", "unit-add", "migrate", "github-workflow-init"} {
		command, present := commands[name]
		if !present {
			t.Fatalf("%s command is missing", name)
		}
		if !reflect.DeepEqual(command.Outputs, []string{"table", "json"}) {
			t.Errorf("%s outputs = %#v, want table and json", name, command.Outputs)
		}
		for _, flag := range command.Flags {
			if flag.Name == "describe" || flag.Name == "verbose" || flag.Name == "output" {
				t.Errorf("%s duplicates inherited Core flag %q", name, flag.Name)
			}
		}
	}
	if outputs := commands["init-options"].Outputs; !reflect.DeepEqual(outputs, []string{"json"}) {
		t.Fatalf("init-options outputs changed outside scope: %#v", outputs)
	}
}

func TestIntegrationDoctorManifestContract(t *testing.T) {
	command, present := loadManifestCommands(t)["doctor"]
	if !present {
		t.Fatal("doctor command is missing")
	}
	if !reflect.DeepEqual(command.Outputs, []string{"table", "json"}) {
		t.Fatalf("outputs = %#v", command.Outputs)
	}
	if len(command.Flags) != 2 || command.Flags[0].Name != "unit" || command.Flags[0].Type != "string" || command.Flags[0].Required ||
		command.Flags[1].Name != "verify-remote" || command.Flags[1].Type != "bool" || command.Flags[1].Required {
		t.Fatalf("doctor flags = %#v", command.Flags)
	}
	for _, forbidden := range []string{"fix", "token", "write", "pipeline", "overview"} {
		if _, present := flagDescriptions(command)[forbidden]; present {
			t.Fatalf("doctor manifest exposes unsupported flag %q", forbidden)
		}
	}
}

func TestValidateManifestClarifiesRepositoryWideValidation(t *testing.T) {
	command, present := loadManifestCommands(t)["validate"]
	if !present {
		t.Fatal("validate command is missing")
	}
	if command.Description != "Validate the complete release configuration" {
		t.Fatalf("validate description = %q", command.Description)
	}
	want := map[string]string{
		"show": "Display structured release configuration details and unit summaries",
		"unit": "Focus displayed V2 unit details; the complete repository is still validated.",
	}
	if got := flagDescriptions(command); !reflect.DeepEqual(got, want) {
		t.Fatalf("validate flag descriptions = %#v, want %#v", got, want)
	}
}

func TestUnitOverviewManifestContract(t *testing.T) {
	command, present := loadManifestCommands(t)["units"]
	if !present {
		t.Fatal("units command is missing")
	}
	if !reflect.DeepEqual(command.Outputs, []string{"table", "json"}) {
		t.Fatalf("outputs = %#v", command.Outputs)
	}
	if len(command.Flags) != 0 {
		t.Fatalf("units flags = %#v, want none", command.Flags)
	}
	for _, forbidden := range []string{"unit", "remote", "deep", "verify", "fix", "repair", "pipeline"} {
		if _, present := flagDescriptions(command)[forbidden]; present {
			t.Fatalf("unit overview manifest exposes unsupported flag %q", forbidden)
		}
	}
}

func TestPipelineManifestContract(t *testing.T) {
	command, present := loadManifestCommands(t)["pipeline"]
	if !present {
		t.Fatal("pipeline command is missing")
	}
	if !reflect.DeepEqual(command.Outputs, []string{"table", "json"}) {
		t.Fatalf("outputs = %#v", command.Outputs)
	}
	if len(command.Flags) != 2 || command.Flags[0].Name != "unit" || command.Flags[0].Type != "string" || command.Flags[0].Required ||
		command.Flags[1].Name != "verify-remote" || command.Flags[1].Type != "bool" || command.Flags[1].Required {
		t.Fatalf("pipeline flags = %#v", command.Flags)
	}
	for _, fragment := range []string{"default human output is concise", "actionable findings", "global --describe", "structured inspection details", "global --verbose", "execution logs", "--verify-remote", "GET-only"} {
		if !strings.Contains(command.Description, fragment) {
			t.Fatalf("pipeline description omitted %q: %q", fragment, command.Description)
		}
	}
	for _, forbidden := range []string{"all", "journal", "resume", "output", "describe", "verbose"} {
		if _, present := flagDescriptions(command)[forbidden]; present {
			t.Fatalf("pipeline manifest exposes unsupported flag %q", forbidden)
		}
	}
}

func TestManifestClarifiesReleaseAndPluginUnitFlagDescriptions(t *testing.T) {
	commands := loadManifestCommands(t)
	initDescriptions := flagDescriptions(commands["init"])

	for _, commandName := range []string{"init", "unit-add"} {
		descriptions := flagDescriptions(commands[commandName])
		for _, flagName := range []string{"kind", "plugin-name", "plugin-manifest", "plugin-asset-prefix", "plugin-binary-name"} {
			if descriptions[flagName] != initDescriptions[flagName] {
				t.Fatalf("%s description for %s is not consistent with init", commandName, flagName)
			}
		}

		assertManifestDescriptionContains(t, commandName, "kind", descriptions["kind"], "release (default)", "normal", "services", "Neko CLI plugins")
		for _, flagName := range []string{"plugin-name", "plugin-manifest", "plugin-asset-prefix", "plugin-binary-name"} {
			assertManifestDescriptionContains(t, commandName, flagName, descriptions[flagName], "Neko CLI plugin", "--kind plugin", "required")
		}
		assertManifestDescriptionContains(t, commandName, "plugin-asset-prefix", descriptions["plugin-asset-prefix"], "must equal unit id")
	}
}

func TestManifestCommandsRouteInMain(t *testing.T) {
	commands := loadManifestCommands(t)
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	re := regexp.MustCompile(`case "([^"]+)"`)
	routed := map[string]bool{}
	for _, match := range re.FindAllStringSubmatch(string(data), -1) {
		routed[match[1]] = true
	}
	if len(routed) != len(commands) {
		t.Fatalf("main routing drifted: routed=%v manifest=%v", sortedKeys(routed), sortedCommandKeys(commands))
	}
	for name := range commands {
		if !routed[name] {
			t.Fatalf("manifest command %q is not routed in main.go", name)
		}
	}
	for name := range routed {
		if _, ok := commands[name]; !ok {
			t.Fatalf("main.go routes public command %q missing from manifest", name)
		}
	}
}

func TestReleaseDocsMentionManifestCommandsAndNoUnsupportedCommands(t *testing.T) {
	commands := loadManifestCommands(t)
	pluginDocs, err := os.ReadFile("../../docs/plugins/release.md")
	if err != nil {
		t.Fatalf("read plugin docs: %v", err)
	}
	referenceDocs, err := os.ReadFile("../../docs/release/cli-reference.md")
	if err != nil {
		t.Fatalf("read cli reference: %v", err)
	}
	combined := string(pluginDocs) + "\n" + string(referenceDocs)
	for name := range commands {
		needle := "neko release " + name
		if !strings.Contains(combined, needle) {
			t.Fatalf("public command %q is missing from release docs", name)
		}
	}
	for _, unsupported := range []string{"neko release dispatch", "neko release retry"} {
		if strings.Contains(combined, unsupported) {
			t.Fatalf("docs mention unsupported public command %q", unsupported)
		}
	}
}

func flagDescriptions(command manifestCommand) map[string]string {
	descriptions := map[string]string{}
	for _, flag := range command.Flags {
		descriptions[flag.Name] = flag.Description
	}
	return descriptions
}

func assertManifestDescriptionContains(t *testing.T, commandName, flagName, description string, fragments ...string) {
	t.Helper()
	if strings.TrimSpace(description) == "" {
		t.Fatalf("%s flag %s has empty description", commandName, flagName)
	}
	for _, fragment := range fragments {
		if !strings.Contains(description, fragment) {
			t.Fatalf("%s flag %s description %q does not contain %q", commandName, flagName, description, fragment)
		}
	}
}

func loadManifestCommands(t *testing.T) map[string]manifestCommand {
	t.Helper()
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Commands []manifestCommand `json:"commands"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	commands := map[string]manifestCommand{}
	for _, command := range manifest.Commands {
		if _, exists := commands[command.Name]; exists {
			t.Fatalf("duplicate manifest command %q", command.Name)
		}
		commands[command.Name] = command
	}
	return commands
}

func stringMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func sortedCommandKeys(commands map[string]manifestCommand) []string {
	keys := make([]string, 0, len(commands))
	for key := range commands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeys(commands map[string]bool) []string {
	keys := make([]string, 0, len(commands))
	for key := range commands {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
