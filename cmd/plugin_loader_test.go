package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/spf13/cobra"
)

func TestPluginRootCommandRendersManifestOverview(t *testing.T) {
	manifest := testPluginManifest("release")
	cmd := CreatePluginCommand(manifest)

	output, err := executeTestCommand(cmd)
	if err != nil {
		t.Fatalf("expected root help to render without dispatch error: %v", err)
	}

	assertContains(t, output, "Plugin: release")
	assertContains(t, output, "Version: 4.0.1")
	assertContains(t, output, "Description: Release management plugin")
	assertContains(t, output, "Available Commands:")
	assertContains(t, output, "validate")
	assertContains(t, output, "patch")
	assertContains(t, output, "Use \"neko release <command> --help\" for command details.")
}

func TestPluginRootHelpFlagRendersManifestOverview(t *testing.T) {
	manifest := testPluginManifest("release")
	cmd := CreatePluginCommand(manifest)

	output, err := executeTestCommand(cmd, "--help")
	if err != nil {
		t.Fatalf("expected --help to render without error: %v", err)
	}

	assertContains(t, output, "Plugin: release")
	assertContains(t, output, "Available Commands:")
	assertContains(t, output, "validate")
}

func TestPluginRootHelpCommandRendersManifestOverview(t *testing.T) {
	manifest := testPluginManifest("release")
	cmd := CreatePluginCommand(manifest)

	output, err := executeTestCommand(cmd, "help")
	if err != nil {
		t.Fatalf("expected help command to render without error: %v", err)
	}

	assertContains(t, output, "Plugin: release")
	assertContains(t, output, "Available Commands:")
	assertContains(t, output, "patch")
}

func TestPluginCommandHelpRendersManifestFlags(t *testing.T) {
	manifest := testPluginManifest("release")
	cmd := CreatePluginCommand(manifest)

	output, err := executeTestCommand(cmd, "patch", "--help")
	if err != nil {
		t.Fatalf("expected command help to render without error: %v", err)
	}

	assertContains(t, output, "Plugin: release")
	assertContains(t, output, "Command: patch")
	assertContains(t, output, "Create a patch release")
	assertContains(t, output, "Outputs: table, json")
	assertContains(t, output, "--unit")
	assertContains(t, output, "string required")
	assertContains(t, output, "--dry-run")
	assertContains(t, output, "bool default=false")
	assertContains(t, output, "Usage: neko release patch [flags]")
}

func TestUnknownPluginCommandReturnsManifestScopedError(t *testing.T) {
	manifest := testPluginManifest("release")
	cmd := CreatePluginCommand(manifest)

	_, err := executeTestCommand(cmd, "does-not-exist")
	if err == nil {
		t.Fatal("expected unknown command error")
	}

	message := err.Error()
	assertContains(t, message, `unknown command "does-not-exist" for plugin "release"`)
	assertContains(t, message, "Available commands:")
	assertContains(t, message, "validate")
	assertContains(t, message, "patch")
	if strings.Contains(message, "unknown command: release") {
		t.Fatalf("unexpected root-command error in message: %s", message)
	}
}

func TestPluginRootHelpDoesNotDispatchPluginBinary(t *testing.T) {
	manifest := testPluginManifest("release")
	pluginDir := installFakePlugin(t, manifest)
	restorePluginDir(t, pluginDir)
	requestPath := filepath.Join(t.TempDir(), "request.json")
	t.Setenv("NEKO_TEST_REQUEST_FILE", requestPath)

	cmd := CreatePluginCommand(manifest)
	output, err := executeTestCommand(cmd)
	if err != nil {
		t.Fatalf("expected root help to render without error: %v", err)
	}

	assertContains(t, output, "Plugin: release")
	if _, err := os.Stat(requestPath); !os.IsNotExist(err) {
		t.Fatalf("expected plugin binary not to be dispatched, stat err: %v", err)
	}
}

func TestPluginSubcommandDispatchesWithManifestCommand(t *testing.T) {
	manifest := testPluginManifest("release")
	pluginDir := installFakePlugin(t, manifest)
	restorePluginDir(t, pluginDir)
	requestPath := filepath.Join(t.TempDir(), "request.json")
	t.Setenv("NEKO_TEST_REQUEST_FILE", requestPath)

	cmd := CreatePluginCommand(manifest)
	_, err := executeTestCommand(cmd, "validate", "--show")
	if err != nil {
		t.Fatalf("expected manifest subcommand to dispatch: %v", err)
	}

	data, err := os.ReadFile(requestPath)
	if err != nil {
		t.Fatalf("expected fake plugin to record request: %v", err)
	}

	var req plugin.Request
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatalf("expected recorded request to be valid JSON: %v", err)
	}
	if req.Command != "validate" {
		t.Fatalf("expected command validate, got %q", req.Command)
	}
	if got := req.Flags["show"]; got != true {
		t.Fatalf("expected show flag to be true, got %#v", got)
	}
}

func TestGetInstalledPluginManifestUsesNekoPluginDir(t *testing.T) {
	manifest := testPluginManifest("generic")
	pluginDir := installFakePlugin(t, manifest)
	t.Setenv("NEKO_PLUGIN_DIR", pluginDir)
	restorePluginDir(t, os.Getenv("NEKO_PLUGIN_DIR"))

	loaded, err := GetInstalledPluginManifest("generic")
	if err != nil {
		t.Fatalf("expected manifest from NEKO_PLUGIN_DIR: %v", err)
	}
	if loaded.Name != "generic" {
		t.Fatalf("expected generic manifest, got %q", loaded.Name)
	}
}

func TestMalformedInstalledPluginManifestFailsClearly(t *testing.T) {
	pluginDir := t.TempDir()
	restorePluginDir(t, pluginDir)
	badPluginDir := filepath.Join(pluginDir, "broken")
	if err := os.MkdirAll(badPluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badPluginDir, "manifest.json"), []byte(`{"name":`), 0644); err != nil {
		t.Fatalf("failed to write malformed manifest: %v", err)
	}

	_, err := GetInstalledPluginManifest("broken")
	if err == nil {
		t.Fatal("expected malformed manifest error")
	}
	assertContains(t, err.Error(), "unexpected end of JSON input")
}

func executeTestCommand(cmd *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func testPluginManifest(name string) plugin.Manifest {
	return plugin.Manifest{
		Name:        name,
		Version:     "4.0.1",
		Description: "Release management plugin",
		Author:      "nekoman-hq",
		Commands: []plugin.Command{
			{
				Name:        "validate",
				Description: "Validate release configuration",
				Outputs:     []string{"table", "json"},
				Flags: []plugin.Flag{
					{Name: "show", Type: "bool", Description: "Show resolved configuration", Required: false, Default: false},
				},
			},
			{
				Name:        "patch",
				Description: "Create a patch release",
				Outputs:     []string{"table", "json"},
				Flags: []plugin.Flag{
					{Name: "unit", Type: "string", Description: "Release unit", Required: true},
					{Name: "dry-run", Type: "bool", Description: "Plan without changing files", Required: false, Default: false},
				},
			},
		},
		RendererTypes: []string{"table", "json"},
	}
}

func installFakePlugin(t *testing.T, manifest plugin.Manifest) string {
	t.Helper()

	pluginDir := t.TempDir()
	installDir := filepath.Join(pluginDir, manifest.Name)
	if err := os.MkdirAll(installDir, 0755); err != nil {
		t.Fatalf("failed to create plugin install dir: %v", err)
	}

	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(installDir, "manifest.json"), manifestData, 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	binaryPath := filepath.Join(installDir, "plugin-"+manifest.Name)
	binary := `#!/bin/sh
cat > "$NEKO_TEST_REQUEST_FILE"
printf '%s\n' '{"status":"success","metadata":{"timestamp":"2026-07-09T00:00:00Z","plugin":"test","version":"1.0.0","command":"validate"},"data":{"message":"dispatched"}}'
`
	if err := os.WriteFile(binaryPath, []byte(binary), 0755); err != nil {
		t.Fatalf("failed to write fake plugin binary: %v", err)
	}

	return pluginDir
}

func restorePluginDir(t *testing.T, pluginDir string) {
	t.Helper()

	oldPluginDir := PluginDir
	PluginDir = pluginDir
	t.Cleanup(func() {
		PluginDir = oldPluginDir
	})
}

func assertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected output to contain %q\noutput:\n%s", needle, haystack)
	}
}
