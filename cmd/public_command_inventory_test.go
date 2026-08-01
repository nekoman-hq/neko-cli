package cmd

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type publicCommandInventoryEntry struct {
	Path       string
	LocalFlags []string
}

var characterizedCoreCommandInventory = []publicCommandInventoryEntry{
	{Path: "neko"},
	{Path: "neko completion"},
	{Path: "neko completion bash", LocalFlags: []string{"no-descriptions"}},
	{Path: "neko completion fish", LocalFlags: []string{"no-descriptions"}},
	{Path: "neko completion powershell", LocalFlags: []string{"no-descriptions"}},
	{Path: "neko completion zsh", LocalFlags: []string{"no-descriptions"}},
	{Path: "neko help"},
	{Path: "neko plugin"},
	{Path: "neko plugin available"},
	{Path: "neko plugin install", LocalFlags: []string{"version"}},
	{Path: "neko plugin list"},
	{Path: "neko plugin uninstall"},
	{Path: "neko plugin update", LocalFlags: []string{"all", "dry-run", "force"}},
	{Path: "neko update", LocalFlags: []string{"dry-run", "force"}},
	{Path: "neko version"},
}

var characterizedFirstPartyPluginInventory = map[string][]publicCommandInventoryEntry{
	"release": {
		{Path: "neko release"},
		{Path: "neko release init", LocalFlags: []string{"delivery", "display-name", "executor", "force", "kind", "paths", "plugin-asset-prefix", "plugin-binary-name", "plugin-manifest", "plugin-name", "tag-prefix", "unit", "version", "workflow", "working-directory"}},
		{Path: "neko release unit-add", LocalFlags: []string{"delivery", "display-name", "executor", "kind", "paths", "plugin-asset-prefix", "plugin-binary-name", "plugin-manifest", "plugin-name", "tag-prefix", "unit", "version", "workflow", "working-directory"}},
		{Path: "neko release init-options"},
		{Path: "neko release migrate", LocalFlags: []string{"dry-run"}},
		{Path: "neko release patch", LocalFlags: []string{"dry-run", "unit"}},
		{Path: "neko release minor", LocalFlags: []string{"dry-run", "unit"}},
		{Path: "neko release major", LocalFlags: []string{"dry-run", "unit"}},
		{Path: "neko release plan", LocalFlags: []string{"change", "unit"}},
		{Path: "neko release doctor", LocalFlags: []string{"unit", "verify-remote"}},
		{Path: "neko release units"},
		{Path: "neko release pipeline", LocalFlags: []string{"unit", "verify-remote"}},
		{Path: "neko release ci-validate-context", LocalFlags: []string{"release-sha", "tag", "unit", "version"}},
		{Path: "neko release github-workflow-init", LocalFlags: []string{"dry-run", "path", "unit"}},
		{Path: "neko release resume", LocalFlags: []string{"dry-run", "unit"}},
		{Path: "neko release history", LocalFlags: []string{"unit"}},
		{Path: "neko release contributors", LocalFlags: []string{"unit"}},
		{Path: "neko release validate", LocalFlags: []string{"show", "unit"}},
		{Path: "neko release evidence", LocalFlags: []string{"family", "identity", "unit"}},
		{Path: "neko release evidence-archive", LocalFlags: []string{"confirm-archive", "digest-sha256", "family", "identity"}},
		{Path: "neko release plugin-index", LocalFlags: []string{"check", "output-file", "pretty", "repository"}},
	},
	"ui": {
		{Path: "neko ui"},
		{Path: "neko ui hello", LocalFlags: []string{"name"}},
		{Path: "neko ui init", LocalFlags: []string{"components-path", "force"}},
		{Path: "neko ui list"},
		{Path: "neko ui add", LocalFlags: []string{"all"}},
		{Path: "neko ui remove", LocalFlags: []string{"all"}},
	},
}

func TestPublicCommandInventoryCharacterizesCoreAndFirstPartyPlugins(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	if len(characterizedCoreCommandInventory) != 15 {
		t.Fatalf("Core command inventory count = %d, want 15", len(characterizedCoreCommandInventory))
	}
	for _, entry := range characterizedCoreCommandInventory {
		command := findCharacterizedCoreCommand(t, entry.Path)
		if len(command.Aliases) != 0 {
			t.Fatalf("%s aliases = %v, want none", entry.Path, command.Aliases)
		}
		if got := localFlagNames(command); !slices.Equal(got, entry.LocalFlags) {
			t.Fatalf("%s local flags = %v, want %v", entry.Path, got, entry.LocalFlags)
		}
	}

	globalFlags := rootCmd.PersistentFlags()
	wantGlobals := map[string]struct {
		shorthand string
		defaultV  string
	}{
		"describe":           {defaultV: "false"},
		"github-output-file": {defaultV: ""},
		"output":             {defaultV: "table"},
		"verbose":            {shorthand: "v", defaultV: "false"},
	}
	if globalFlags.NFlag() != 0 {
		t.Fatal("global defaults must not be marked changed during inventory")
	}
	globalNames := make([]string, 0)
	globalFlags.VisitAll(func(flag *pflag.Flag) {
		globalNames = append(globalNames, flag.Name)
	})
	if !reflect.DeepEqual(globalNames, []string{"describe", "github-output-file", "output", "verbose"}) {
		t.Fatalf("global plugin-response flags = %v", globalNames)
	}
	seenGlobals := map[string]bool{}
	for name, expected := range wantGlobals {
		flag := globalFlags.Lookup(name)
		if flag == nil {
			t.Errorf("global flag --%s is missing", name)
			continue
		}
		seenGlobals[name] = true
		if flag.Shorthand != expected.shorthand || flag.DefValue != expected.defaultV {
			t.Errorf("global --%s shorthand/default = %q/%q, want %q/%q", name, flag.Shorthand, flag.DefValue, expected.shorthand, expected.defaultV)
		}
	}
	if len(seenGlobals) != 4 {
		t.Fatalf("global plugin-response flag count = %d, want 4", len(seenGlobals))
	}

	pluginPathCount := 0
	for pluginName, expected := range characterizedFirstPartyPluginInventory {
		manifest := loadFirstPartyManifest(t, pluginName)
		if len(manifest.Commands)+1 != len(expected) {
			t.Fatalf("%s public path count = %d, want %d", pluginName, len(manifest.Commands)+1, len(expected))
		}
		if strings.Contains(readFirstPartyManifest(t, pluginName), `"alias`) {
			t.Fatalf("%s manifest unexpectedly declares command aliases", pluginName)
		}

		for index, entry := range expected[1:] {
			command := manifest.Commands[index]
			if entry.Path != "neko "+pluginName+" "+command.Name {
				t.Fatalf("%s manifest command %d = %q, inventory path = %q", pluginName, index, command.Name, entry.Path)
			}
			gotFlags := make([]string, 0, len(command.Flags))
			for _, flag := range command.Flags {
				gotFlags = append(gotFlags, flag.Name)
			}
			sort.Strings(gotFlags)
			if !slices.Equal(gotFlags, entry.LocalFlags) {
				t.Fatalf("%s local flags = %v, want %v", entry.Path, gotFlags, entry.LocalFlags)
			}
		}
		pluginPathCount += len(expected)
	}

	if got := len(characterizedCoreCommandInventory) + pluginPathCount; got != 42 {
		t.Fatalf("complete characterized public command count = %d, want 42", got)
	}
}

func TestUIManifestCharacterizesUnroutedHelloCommand(t *testing.T) {
	manifest := loadFirstPartyManifest(t, "ui")
	manifestNames := make([]string, 0, len(manifest.Commands))
	for _, command := range manifest.Commands {
		manifestNames = append(manifestNames, command.Name)
	}
	if !reflect.DeepEqual(manifestNames, []string{"hello", "init", "list", "add", "remove"}) {
		t.Fatalf("UI manifest commands = %v", manifestNames)
	}

	router, err := os.ReadFile("../plugin/ui/main.go")
	if err != nil {
		t.Fatalf("read UI router: %v", err)
	}
	for _, routed := range []string{"init", "list", "add", "remove"} {
		if strings.Count(string(router), `case "`+routed+`":`) != 1 {
			t.Fatalf("UI router command %q count drifted", routed)
		}
	}
	if strings.Contains(string(router), `case "hello":`) {
		t.Fatal("UI hello unexpectedly became routed; update its documented support status")
	}
}

func findCharacterizedCoreCommand(t *testing.T, path string) *cobra.Command {
	t.Helper()
	parts := strings.Fields(path)
	if len(parts) == 1 {
		return rootCmd
	}
	command, remaining, err := rootCmd.Find(parts[1:])
	if err != nil || len(remaining) != 0 || command.CommandPath() != path {
		t.Fatalf("find %s: command=%v remaining=%v err=%v", path, command, remaining, err)
	}
	return command
}

func localFlagNames(command *cobra.Command) []string {
	command.InitDefaultHelpFlag()
	names := make([]string, 0)
	command.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name != "help" {
			names = append(names, flag.Name)
		}
	})
	sort.Strings(names)
	return names
}

func loadFirstPartyManifest(t *testing.T, name string) plugin.Manifest {
	t.Helper()
	data := []byte(readFirstPartyManifest(t, name))
	var manifest plugin.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode %s manifest: %v", name, err)
	}
	if manifest.Name != name {
		t.Fatalf("manifest %s name = %q", name, manifest.Name)
	}
	return manifest
}

func readFirstPartyManifest(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../plugin/" + name + "/manifest.json")
	if err != nil {
		t.Fatalf("read %s manifest: %v", name, err)
	}
	return string(data)
}
