package main

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type manifestCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Flags       []struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
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
			"family": "string",
			"unit":   "string",
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
