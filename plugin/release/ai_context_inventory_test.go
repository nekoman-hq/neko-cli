package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAIContextHasOneCurrentEntryPoint(t *testing.T) {
	root := repositoryRoot(t)
	contextPath := filepath.Join(root, "docs", "ai_context.md")
	info, err := os.Stat(contextPath)
	if err != nil {
		t.Fatalf("stat AI context: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatal("docs/ai_context.md is not a regular file")
	}

	var entries []string
	for _, path := range trackedFiles(t, root) {
		if strings.EqualFold(filepath.Base(path), "ai_context.md") {
			entries = append(entries, path)
		}
	}
	if len(entries) != 1 || entries[0] != "docs/ai_context.md" {
		t.Fatalf("tracked AI context entry points = %v, want [docs/ai_context.md]", entries)
	}

	pluginDevelopment := readDocumentationFile(t, filepath.Join(root, "docs", "plugin-development.md"))
	if !strings.Contains(pluginDevelopment, "](ai_context.md)") {
		t.Error("plugin development guide no longer links to the current AI context entry point")
	}
}

func TestAIContextCurrentSourcesRemainAvailable(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range []string{
		"docs/cli-reference.md",
		"docs/release/cli-reference.md",
		"docs/release/compatibility.md",
		"docs/release/migration-v1-to-v2.md",
		"docs/installation.md",
		"plugin/release/manifest.json",
		"plugin/release/docs/architecture/current-state.md",
		"plugin/release/docs/architecture/package-ownership.md",
		"plugin/release/docs/architecture/architecture-decisions.md",
		"plugin/release/docs/architecture/maintainability-policy.md",
		"plugin/release/docs/architecture/compatibility-notes.md",
		"plugin/release/docs/history/README.md",
	} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Errorf("current AI-context source %s is missing: %v", path, err)
			continue
		}
		if !info.Mode().IsRegular() {
			t.Errorf("current AI-context source %s is not a regular file", path)
		}
	}
}
