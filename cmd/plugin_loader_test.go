package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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
	assertNotContains(t, output, "Neko CLI plugin unit flags")
}

func TestPluginCommandHelpSeparatesInheritedGlobalPluginResponseFlags(t *testing.T) {
	manifest := testPluginManifest("release")
	root := testRootWithPluginResponseFlags()
	root.AddCommand(CreatePluginCommand(manifest))

	output, err := executeTestCommand(root, "release", "patch", "--help")
	if err != nil {
		t.Fatalf("expected command help to render without error: %v", err)
	}

	commandIndex := strings.Index(output, "\nCommand flags:\n")
	globalIndex := strings.Index(output, "\nGlobal plugin-response flags:\n")
	usageIndex := strings.Index(output, "\nUsage:")
	if commandIndex < 0 || globalIndex < 0 || usageIndex < 0 || commandIndex >= globalIndex || globalIndex >= usageIndex {
		t.Fatalf("expected separated command/global flag sections before usage, got:\n%s", output)
	}
	commandSection := output[commandIndex:globalIndex]
	globalSection := output[globalIndex:usageIndex]
	for _, name := range []string{"--unit", "--dry-run"} {
		assertContains(t, commandSection, name)
		assertNotContains(t, globalSection, name)
	}
	for _, name := range []string{"--describe", "--verbose", "--output", "--github-output-file"} {
		assertNotContains(t, commandSection, name)
		assertContains(t, globalSection, name)
		if count := countHelpFlagDefinitions(output, name); count != 1 {
			t.Fatalf("help occurrence count for %s = %d, want 1:\n%s", name, count, output)
		}
	}
	assertOrderedHelpFlags(t, commandSection, "--unit", "--dry-run")
	assertOrderedHelpFlags(t, globalSection, "--describe", "--verbose", "--output", "--github-output-file")
}

func TestPluginIndexHelpKeepsKnownLocalGlobalOutputCollisionVisible(t *testing.T) {
	manifest := plugin.Manifest{
		Name: "release",
		Commands: []plugin.Command{{
			Name: "plugin-index",
			Flags: []plugin.Flag{
				{Name: "output", Type: "string", Description: "Optional file path to write plugin-index.json"},
				{Name: "check", Type: "bool", Description: "Check only"},
			},
		}},
	}
	root := testRootWithPluginResponseFlags()
	root.AddCommand(CreatePluginCommand(manifest))

	output, err := executeTestCommand(root, "release", "plugin-index", "--help")
	if err != nil {
		t.Fatalf("expected plugin-index help to render without error: %v", err)
	}
	commandIndex := strings.Index(output, "\nCommand flags:\n")
	globalIndex := strings.Index(output, "\nGlobal plugin-response flags:\n")
	if commandIndex < 0 || globalIndex < 0 {
		t.Fatalf("expected separated flag sections, got:\n%s", output)
	}
	commandSection := output[commandIndex:globalIndex]
	globalSection := output[globalIndex:]
	assertContains(t, commandSection, "--output")
	assertContains(t, commandSection, "Optional file path to write plugin-index.json")
	if count := countHelpFlagDefinitions(globalSection, "--output"); count != 0 {
		t.Fatalf("global plugin-index help defines shadowed --output %d times:\n%s", count, globalSection)
	}
	for _, name := range []string{"--describe", "--verbose", "--github-output-file"} {
		assertContains(t, globalSection, name)
	}
	if count := countHelpFlagDefinitions(output, "--output"); count != 1 {
		t.Fatalf("plugin-index --output occurrence count = %d, want one local collision surface:\n%s", count, output)
	}
}

func TestPluginIndexCollisionSerializesShadowedOutputLocally(t *testing.T) {
	root := testRootWithPluginResponseFlags()
	command := &cobra.Command{Use: "plugin-index"}
	command.Flags().String("output", "", "Optional file path to write plugin-index.json")
	var got map[string]any
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		got = extractFlags(cmd)
		return nil
	}
	root.AddCommand(command)
	root.SetArgs([]string{"plugin-index", "--output", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute plugin-index collision characterization: %v", err)
	}

	want := map[string]any{"output": "json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extracted collision flags = %#v, want %#v", got, want)
	}
}

func TestCoreCommandHelpDoesNotReceiveCustomPluginFlagSections(t *testing.T) {
	root := testRootWithPluginResponseFlags()
	root.AddCommand(&cobra.Command{Use: "version", Short: "Show CLI version"})

	output, err := executeTestCommand(root, "version", "--help")
	if err != nil {
		t.Fatalf("expected Core command help to render without error: %v", err)
	}
	assertNotContains(t, output, "Command flags:")
	assertNotContains(t, output, "Global plugin-response flags:")
	assertContains(t, output, "Show CLI version")
}

func TestExtractFlagsSerializesOnlyCommandLocalNonPersistentFlags(t *testing.T) {
	root := testRootWithPluginResponseFlags()
	command := &cobra.Command{Use: "inspect"}
	command.Flags().String("unit", "", "Release unit")
	command.Flags().Bool("show", false, "Show details")
	var got map[string]any
	command.RunE = func(cmd *cobra.Command, _ []string) error {
		got = extractFlags(cmd)
		return nil
	}
	root.AddCommand(command)
	root.SetArgs([]string{"inspect", "--unit", "cli", "--show", "--describe", "--verbose", "--output", "json", "--github-output-file", "/private/tmp/neko-cli-output"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute flag extraction command: %v", err)
	}

	want := map[string]any{"unit": "cli", "show": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extracted flags = %#v, want %#v", got, want)
	}
}

func TestPluginCommandHelpGroupsPluginUnitFlags(t *testing.T) {
	manifest := plugin.Manifest{
		Name:        "release",
		Version:     "4.0.1",
		Description: "Release management plugin",
		Commands: []plugin.Command{
			{
				Name:        "init",
				Description: "Initialize release configuration",
				Outputs:     []string{"text", "json"},
				Flags: []plugin.Flag{
					{Name: "unit", Type: "string", Description: "Release unit id", Required: false, Default: "cli"},
					{Name: "kind", Type: "string", Description: "Unit kind", Required: false, Default: "release"},
					{Name: "plugin-name", Type: "string", Description: "Public Neko CLI plugin name; only for --kind plugin", Required: false},
					{Name: "plugin-manifest", Type: "string", Description: "Neko CLI plugin manifest path; only for --kind plugin", Required: false},
				},
			},
		},
	}
	cmd := CreatePluginCommand(manifest)

	output, err := executeTestCommand(cmd, "init", "--help")
	if err != nil {
		t.Fatalf("expected command help to render without error: %v", err)
	}

	flagsIndex := strings.Index(output, "\nCommand flags:\n")
	pluginIndex := strings.Index(output, "\nNeko CLI plugin unit flags (only with --kind plugin):\n")
	if flagsIndex < 0 || pluginIndex < 0 || flagsIndex >= pluginIndex {
		t.Fatalf("expected plugin flags after normal flags section, got:\n%s", output)
	}
	normalSection := output[flagsIndex:pluginIndex]
	pluginSection := output[pluginIndex:]
	assertContains(t, normalSection, "--unit")
	assertContains(t, normalSection, "--kind")
	assertContains(t, normalSection, "string default=release")
	assertNotContains(t, normalSection, "--plugin-name")
	assertContains(t, pluginSection, "--plugin-name")
	assertContains(t, pluginSection, "Public Neko CLI plugin name")
	assertContains(t, pluginSection, "--plugin-manifest")
	assertContains(t, pluginSection, "only for --kind plugin")
	assertContains(t, output, "Usage: neko release init [flags]")

	initCmd, _, err := cmd.Find([]string{"init"})
	if err != nil {
		t.Fatalf("find init command: %v", err)
	}
	for _, name := range []string{"unit", "kind", "plugin-name", "plugin-manifest"} {
		if initCmd.Flags().Lookup(name) == nil {
			t.Fatalf("expected registered flag %q", name)
		}
	}
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

func TestRequiredReleaseContextFlagsFailBeforeDispatchWhenMissing(t *testing.T) {
	command := &cobra.Command{Use: "ci-validate-context"}
	definitions := []plugin.Flag{
		{Name: "unit", Type: "string", Required: true},
		{Name: "version", Type: "string", Required: true},
		{Name: "tag", Type: "string", Required: true},
		{Name: "release-sha", Type: "string", Required: true},
	}
	for _, definition := range definitions {
		command.Flags().String(definition.Name, "", definition.Description)
	}

	err := validateRequiredFlagsFromManifest(command, definitions)
	if err == nil {
		t.Fatal("missing release context flags unexpectedly passed")
	}
	for _, name := range []string{"unit", "version", "tag", "release-sha"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("missing flag error omitted %s: %v", name, err)
		}
		if setErr := command.Flags().Set(name, "value"); setErr != nil {
			t.Fatalf("set %s: %v", name, setErr)
		}
	}
	if err := validateRequiredFlagsFromManifest(command, definitions); err != nil {
		t.Fatalf("complete release context flags failed: %v", err)
	}
}

func executeTestCommand(cmd *cobra.Command, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func testRootWithPluginResponseFlags() *cobra.Command {
	root := &cobra.Command{Use: "neko"}
	root.PersistentFlags().Bool("describe", false, "Include structured details and metadata in output -- only for plugin responses")
	root.PersistentFlags().BoolP("verbose", "v", false, "Include execution and debug logs in plugin output")
	root.PersistentFlags().String("output", "table", "Output format (table, json, wide, github) -- only for plugin responses")
	root.PersistentFlags().String("github-output-file", "", "Explicit GitHub Actions command-file destination -- only for --output github")
	return root
}

func assertOrderedHelpFlags(t *testing.T, output string, names ...string) {
	t.Helper()
	last := -1
	for _, name := range names {
		index := strings.Index(output, name)
		if index < 0 || index <= last {
			t.Fatalf("flags are not in expected order %v:\n%s", names, output)
		}
		last = index
	}
}

func countHelpFlagDefinitions(output, name string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[0] == name {
			count++
		}
	}
	return count
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

func assertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected output not to contain %q\noutput:\n%s", needle, haystack)
	}
}
