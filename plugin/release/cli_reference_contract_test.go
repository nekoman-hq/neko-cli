package main

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const canonicalReleaseCLIReference = "../../docs/release/cli-reference.md"

func TestCanonicalReleaseReferenceHasOneSupportRowPerPublicPath(t *testing.T) {
	document := readCanonicalReleaseReference(t)
	rows := releaseDocumentationTableRows(t, document, "release-support-matrix")
	commands := loadManifestCommands(t)

	wantSupport := map[string]string{
		"neko release":                      "Core overview",
		"neko release init":                 "V2 only",
		"neko release unit-add":             "V2 only",
		"neko release init-options":         "V2 only",
		"neko release migrate":              "V1 to V2 migration",
		"neko release patch":                "Shared V1/V2",
		"neko release minor":                "Shared V1/V2",
		"neko release major":                "Shared V1/V2",
		"neko release plan":                 "Shared V1/V2",
		"neko release doctor":               "V2 only",
		"neko release units":                "V2 only",
		"neko release pipeline":             "V2 only",
		"neko release ci-validate-context":  "V2 only",
		"neko release github-workflow-init": "V2 only",
		"neko release resume":               "V2 only",
		"neko release history":              "Shared V1/V2",
		"neko release contributors":         "Shared V1/V2",
		"neko release validate":             "Shared V1/V2",
		"neko release evidence":             "Shared V1/V2",
		"neko release evidence-archive":     "Shared V1/V2",
		"neko release plugin-index":         "V2 only",
	}
	if len(rows) != len(wantSupport) {
		t.Fatalf("Release support row count = %d, want %d", len(rows), len(wantSupport))
	}

	seen := map[string]bool{}
	for _, row := range rows {
		if len(row) != 13 {
			t.Fatalf("Release support row has %d columns, want 13: %v", len(row), row)
		}
		path := releaseMarkdownCodeValue(row[0])
		if seen[path] {
			t.Fatalf("Release path %q has duplicate support rows", path)
		}
		seen[path] = true
		if got, ok := wantSupport[path]; !ok || row[1] != got {
			t.Errorf("Release path %q support = %q, want %q", path, row[1], got)
		}
		for index, label := range []string{"files", "read/mutate", "network", "token", "Git", "filesystem", "manifest outputs", "default", "describe", "verbose", "exit"} {
			if strings.TrimSpace(row[index+2]) == "" {
				t.Errorf("Release path %q has empty %s classification", path, label)
			}
		}

		if path == "neko release" {
			continue
		}
		name := strings.TrimPrefix(path, "neko release ")
		command, ok := commands[name]
		if !ok {
			t.Errorf("support matrix documents ghost Release command %q", name)
			continue
		}
		if got, want := releaseMarkdownCodeValue(row[8]), strings.Join(command.Outputs, ", "); got != want {
			t.Errorf("%s manifest outputs = %q, want %q", path, got, want)
		}
	}
	for path := range wantSupport {
		if !seen[path] {
			t.Errorf("Release support matrix omitted %q", path)
		}
	}
}

func TestCanonicalReleaseReferenceAccountsForEveryManifestLocalFlag(t *testing.T) {
	document := readCanonicalReleaseReference(t)
	rows := releaseDocumentationTableRows(t, document, "release-local-flag-inventory")
	commands := loadManifestCommands(t)

	want := make([]string, 0)
	for name, command := range commands {
		for _, flag := range command.Flags {
			want = append(want, "neko release "+name+"|--"+flag.Name+"|"+flag.Type+"|"+fmt.Sprint(flag.Required))
		}
	}
	sort.Strings(want)

	got := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) != 8 {
			t.Fatalf("Release local flag row has %d columns, want 8: %v", len(row), row)
		}
		path := releaseMarkdownCodeValue(row[0])
		flag := releaseMarkdownCodeValue(row[1])
		got = append(got, path+"|"+flag+"|"+row[2]+"|"+row[3])
		if strings.TrimSpace(row[4]) == "" || strings.TrimSpace(row[5]) == "" {
			t.Errorf("%s %s omits default or accepted-value/restriction facts", path, flag)
		}
		if row[6] != "Last wins" {
			t.Errorf("%s %s repeat policy = %q, want Last wins", path, flag, row[6])
		}
		if row[7] != "Release manifest" {
			t.Errorf("%s %s owner = %q, want Release manifest", path, flag, row[7])
		}
		for _, global := range []string{"--describe", "--verbose", "--output", "--github-output-file"} {
			if flag == global {
				t.Errorf("%s incorrectly documents inherited %s as a local flag", path, flag)
			}
		}
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("documented Release local flags =\n%v\nwant =\n%v", got, want)
	}

	for _, rule := range []string{
		"No public Release command or flag is deprecated.",
		"There are no public Release command aliases or compatibility flag aliases.",
		"`--project-type`, `--release-system`, and `--metadata` are not registered public flags",
		"Use `--output-file` to persist Plugin Index bytes",
		"`--check` and `--output-file` are mutually exclusive",
	} {
		if !strings.Contains(document, rule) {
			t.Fatalf("canonical Release reference omitted compatibility rule %q", rule)
		}
	}
}

func TestCanonicalReleaseReferenceSeparatesGlobalFlagsAndOutputVocabulary(t *testing.T) {
	document := readCanonicalReleaseReference(t)
	rows := releaseDocumentationTableRows(t, document, "release-global-flag-inventory")
	want := []string{
		"--help, -h|Cobra|false",
		"--describe|Core|false",
		"--verbose, -v|Core|false",
		"--output|Core|table",
		"--github-output-file|Core|empty",
	}
	got := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) != 8 {
			t.Fatalf("Release global flag row has %d columns, want 8: %v", len(row), row)
		}
		got = append(got, releaseMarkdownCodeValue(row[0])+"|"+row[1]+"|"+releaseMarkdownCodeValue(row[3]))
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("documented Release global flags = %v, want %v", got, want)
	}

	for _, rule := range []string{
		"Core accepts exactly `table`, `json`, `wide`, and `github`",
		"Only `ci-validate-context` declares successful GitHub command-file output",
		"`--describe` cannot be combined with `--output github`",
		"explicit response exit `0` through `125`",
		"Pipeline `blocked` -> `0`",
		"Plugin Index failed check -> `1`",
	} {
		if !strings.Contains(document, rule) {
			t.Fatalf("canonical Release reference omitted output/exit rule %q", rule)
		}
	}
}

func readCanonicalReleaseReference(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(canonicalReleaseCLIReference)
	if err != nil {
		t.Fatalf("read canonical Release CLI reference: %v", err)
	}
	return string(data)
}

func releaseDocumentationTableRows(t *testing.T, document, name string) [][]string {
	t.Helper()
	start := "<!-- " + name + ":start -->"
	end := "<!-- " + name + ":end -->"
	startIndex := strings.Index(document, start)
	endIndex := strings.Index(document, end)
	if startIndex < 0 || endIndex < 0 || endIndex <= startIndex {
		t.Fatalf("Release documentation table markers %q are missing or out of order", name)
	}

	lines := strings.Split(document[startIndex+len(start):endIndex], "\n")
	rows := make([][]string, 0)
	headerSeen := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || strings.Contains(trimmed, "---") {
			continue
		}
		parts := strings.Split(strings.Trim(trimmed, "|"), "|")
		for index := range parts {
			parts[index] = strings.TrimSpace(parts[index])
		}
		if !headerSeen {
			headerSeen = true
			continue
		}
		rows = append(rows, parts)
	}
	if !headerSeen {
		t.Fatalf("Release documentation table %q has no header", name)
	}
	return rows
}

func releaseMarkdownCodeValue(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`")
}
