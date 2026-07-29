package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

type releaseCommandOutputInventoryEntry struct {
	Name           string
	Scope          string
	Access         string
	Network        string
	Response       string
	Classification string
	Outputs        []string
}

var characterizedReleaseCommandOutputInventory = []releaseCommandOutputInventoryEntry{
	{Name: "init", Scope: "V2", Access: "mutating", Network: "local-only", Response: "structured", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "unit-add", Scope: "V2", Access: "mutating", Network: "local-only", Response: "structured", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "init-options", Scope: "V2", Access: "read-only", Network: "local-only", Response: "structured", Classification: "neither", Outputs: []string{"table", "json"}},
	{Name: "migrate", Scope: "shared", Access: "mutating", Network: "local-only", Response: "structured", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "patch", Scope: "shared", Access: "mutating", Network: "optionally-remote", Response: "lifecycle", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "minor", Scope: "shared", Access: "mutating", Network: "optionally-remote", Response: "lifecycle", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "major", Scope: "shared", Access: "mutating", Network: "optionally-remote", Response: "lifecycle", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "plan", Scope: "shared", Access: "read-only", Network: "local-only", Response: "structured", Classification: "describe-only", Outputs: []string{"table", "json"}},
	{Name: "doctor", Scope: "V2", Access: "read-only", Network: "explicit-remote-read", Response: "structured", Classification: "describe-only", Outputs: []string{"table", "json"}},
	{Name: "units", Scope: "V2", Access: "read-only", Network: "local-only", Response: "structured", Classification: "describe-only", Outputs: []string{"table", "json"}},
	{Name: "pipeline", Scope: "V2", Access: "read-only", Network: "explicit-remote-read", Response: "structured-v1", Classification: "describe-only", Outputs: []string{"table", "json"}},
	{Name: "ci-validate-context", Scope: "V2", Access: "read-only", Network: "local-only", Response: "structured", Classification: "describe-only", Outputs: []string{"table", "json", "github"}},
	{Name: "github-workflow-init", Scope: "V2", Access: "mutating", Network: "local-only", Response: "structured", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "resume", Scope: "V2", Access: "mutating", Network: "optionally-remote", Response: "lifecycle", Classification: "describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "history", Scope: "shared", Access: "read-only", Network: "local-only", Response: "structured", Classification: "neither", Outputs: []string{"table", "json"}},
	{Name: "contributors", Scope: "shared", Access: "read-only", Network: "local-only", Response: "structured", Classification: "neither", Outputs: []string{"table", "json"}},
	{Name: "validate", Scope: "shared", Access: "read-only", Network: "local-only", Response: "mode-sensitive-structured", Classification: "describe-only", Outputs: []string{"table", "json"}},
	{Name: "evidence", Scope: "shared", Access: "read-only", Network: "local-only", Response: "legacy-structured", Classification: "describe-only", Outputs: []string{"table", "json"}},
	{Name: "evidence-archive", Scope: "shared", Access: "guarded-mutation", Network: "local-only", Response: "legacy-structured", Classification: "limited-describe+verbose", Outputs: []string{"table", "json"}},
	{Name: "plugin-index", Scope: "V2", Access: "mode-dependent", Network: "local-only", Response: "raw-or-structured-v1", Classification: "mode-dependent", Outputs: []string{"json", "table"}},
}

func TestReleaseCommandOutputInventoryCharacterizesManifestAndRouting(t *testing.T) {
	manifestData, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Commands []manifestCommand `json:"commands"`
	}
	if decodeErr := json.Unmarshal(manifestData, &manifest); decodeErr != nil {
		t.Fatalf("decode manifest: %v", decodeErr)
	}

	if len(manifest.Commands) != 20 {
		t.Fatalf("manifest command count = %d, want 20", len(manifest.Commands))
	}
	if len(characterizedReleaseCommandOutputInventory) != len(manifest.Commands) {
		t.Fatalf(
			"output inventory count = %d, manifest count = %d",
			len(characterizedReleaseCommandOutputInventory),
			len(manifest.Commands),
		)
	}

	for index, expected := range characterizedReleaseCommandOutputInventory {
		command := manifest.Commands[index]
		if command.Name != expected.Name {
			t.Fatalf("manifest command %d = %q, want %q", index, command.Name, expected.Name)
		}
		if !reflect.DeepEqual(command.Outputs, expected.Outputs) {
			t.Fatalf("%s outputs = %v, characterized %v", command.Name, command.Outputs, expected.Outputs)
		}
		for _, value := range []string{
			expected.Scope,
			expected.Access,
			expected.Network,
			expected.Response,
			expected.Classification,
		} {
			if strings.TrimSpace(value) == "" {
				t.Fatalf("%s has an incomplete output inventory entry", command.Name)
			}
		}
	}

	mainData, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main routing: %v", err)
	}
	for _, command := range manifest.Commands {
		route := `case "` + command.Name + `":`
		if strings.Count(string(mainData), route) != 1 {
			t.Fatalf("command %q route count = %d, want 1", command.Name, strings.Count(string(mainData), route))
		}
	}
}

func TestReleaseManifestCharacterizesNoAliasesAndGlobalFlagOwnership(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if strings.Contains(string(data), `"aliases"`) || strings.Contains(string(data), `"alias"`) {
		t.Fatal("Release manifest unexpectedly declares aliases")
	}

	for name, command := range loadManifestCommands(t) {
		seen := map[string]bool{}
		for _, flag := range command.Flags {
			if seen[flag.Name] {
				t.Fatalf("%s declares duplicate local flag %q", name, flag.Name)
			}
			seen[flag.Name] = true
			switch flag.Name {
			case "describe", "verbose", "output", "github-output-file":
				t.Fatalf("%s shadows Core-owned global flag %q", name, flag.Name)
			}
		}
	}
}

func TestReleaseManifestUsesOnlySelectableRendererDeclarations(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		RendererTypes []string `json:"renderer_types"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if !reflect.DeepEqual(manifest.RendererTypes, []string{"table", "json", "github"}) {
		t.Fatalf("renderer types = %v", manifest.RendererTypes)
	}
}
