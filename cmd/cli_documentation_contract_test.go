package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const publicCLIReferencePath = "../docs/cli-reference.md"

func TestPublicCLIReferenceAccountsForEveryCharacterizedCommand(t *testing.T) {
	document := readDocumentationContractFile(t, publicCLIReferencePath)
	rows := documentationTableRows(t, document, "public-command-inventory")

	want := make([]string, 0, 42)
	for _, entry := range characterizedCoreCommandInventory {
		want = append(want, entry.Path)
	}
	for _, pluginName := range []string{"release", "ui"} {
		for _, entry := range characterizedFirstPartyPluginInventory[pluginName] {
			want = append(want, entry.Path)
		}
	}
	sort.Strings(want)

	got := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if len(row) < 7 {
			t.Fatalf("public command row has %d columns, want at least 7: %v", len(row), row)
		}
		path := documentedCommandPath(row[0])
		if seen[path] {
			t.Fatalf("public command %q is documented more than once", path)
		}
		seen[path] = true
		got = append(got, path)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("documented public commands =\n%v\nwant =\n%v", got, want)
	}
	if !strings.Contains(document, "Command aliases: none.") {
		t.Fatal("public CLI reference does not state that there are no command aliases")
	}
}

func TestPublicCLIReferenceAccountsForCoreAndUIFlags(t *testing.T) {
	document := readDocumentationContractFile(t, publicCLIReferencePath)
	rows := documentationTableRows(t, document, "public-flag-inventory")
	want := []string{
		"all commands|--help, -h",
		"plugin response|--describe",
		"plugin response|--github-output-file",
		"plugin response|--output",
		"plugin response|--verbose, -v",
		"neko completion bash|--no-descriptions",
		"neko completion fish|--no-descriptions",
		"neko completion powershell|--no-descriptions",
		"neko completion zsh|--no-descriptions",
		"neko update|--dry-run",
		"neko update|--force",
		"neko plugin install|--version",
		"neko plugin update|--all",
		"neko plugin update|--dry-run",
		"neko plugin update|--force",
		"neko ui hello|--name",
		"neko ui init|--components-path",
		"neko ui init|--force",
		"neko ui add|--all",
		"neko ui remove|--all",
	}

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) < 8 {
			t.Fatalf("public flag row has %d columns, want at least 8: %v", len(row), row)
		}
		got = append(got, markdownCodeValue(row[0])+"|"+markdownCodeValue(row[1]))
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("documented Core/UI flags =\n%v\nwant =\n%v", got, want)
	}

	for _, required := range []string{
		"Core `neko update --force` is only a same-version reinstall switch",
		"does not bypass permissions",
		"does not permit a Core downgrade",
		"scalar flags may be repeated; the last occurrence wins",
	} {
		if !strings.Contains(document, required) {
			t.Fatalf("public CLI reference omitted flag rule %q", required)
		}
	}
}

func TestUserFacingExamplesDoNotRestoreStaleReleaseSyntax(t *testing.T) {
	paths := userFacingMarkdownPaths(t)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "neko ") {
				continue
			}
			if strings.HasPrefix(trimmed, "neko release plugin-index --output ") &&
				!strings.HasPrefix(trimmed, "neko release plugin-index --output json") &&
				!strings.HasPrefix(trimmed, "neko release plugin-index --output table") &&
				!strings.HasPrefix(trimmed, "neko release plugin-index --output wide") &&
				!strings.HasPrefix(trimmed, "neko release plugin-index --output github") {
				t.Errorf("%s:%d uses stale Plugin Index output-path syntax: %s", path, lineNumber+1, trimmed)
			}
			for _, staleFlag := range []string{"--project-type", "--release-system", "--metadata", "--verify_remote", "--dryrun", "--dry_run"} {
				if strings.Contains(trimmed, staleFlag) {
					t.Errorf("%s:%d uses stale public flag %s: %s", path, lineNumber+1, staleFlag, trimmed)
				}
			}
		}
	}
}

func readDocumentationContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read documentation contract %s: %v", path, err)
	}
	return string(data)
}

func documentationTableRows(t *testing.T, document, name string) [][]string {
	t.Helper()
	start := "<!-- " + name + ":start -->"
	end := "<!-- " + name + ":end -->"
	startIndex := strings.Index(document, start)
	endIndex := strings.Index(document, end)
	if startIndex < 0 || endIndex < 0 || endIndex <= startIndex {
		t.Fatalf("documentation table markers %q are missing or out of order", name)
	}

	rows := make([][]string, 0)
	for _, line := range strings.Split(document[startIndex+len(start):endIndex], "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || strings.Contains(trimmed, "---") {
			continue
		}
		parts := strings.Split(strings.Trim(trimmed, "|"), "|")
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
		}
		if len(rows) == 0 {
			rows = append(rows, nil)
			continue
		}
		rows = append(rows, parts)
	}
	if len(rows) == 0 {
		t.Fatalf("documentation table %q has no header", name)
	}
	return rows[1:]
}

func markdownCodeValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`")
}

func documentedCommandPath(value string) string {
	parts := strings.Fields(markdownCodeValue(value))
	path := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, "[") || strings.HasPrefix(part, "<") {
			break
		}
		path = append(path, part)
	}
	return strings.Join(path, " ")
}

func userFacingMarkdownPaths(t *testing.T) []string {
	t.Helper()
	paths := []string{"../README.md"}
	err := filepath.Walk("../docs", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk user-facing documentation: %v", err)
	}
	sort.Strings(paths)
	return paths
}
