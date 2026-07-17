package pluginindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePluginIndexOutputTargetUsesRepositoryRootForRelativePaths(t *testing.T) {
	root := t.TempDir()
	target, err := resolvePluginIndexOutputTarget(root, "nested/plugin-index.json", nil)
	if err != nil {
		t.Fatalf("resolvePluginIndexOutputTarget: %v", err)
	}
	want := filepath.Join(root, "nested", "plugin-index.json")
	if target.AbsolutePath != want || target.ConfiguredPath != "nested/plugin-index.json" || target.External {
		t.Fatalf("target = %#v, want absolute %s with configured display path", target, want)
	}
}

func TestResolvePluginIndexOutputTargetKeepsExplicitAbsoluteTargets(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(t.TempDir(), "plugin-index.json")
	target, err := resolvePluginIndexOutputTarget(root, output, nil)
	if err != nil {
		t.Fatalf("resolvePluginIndexOutputTarget: %v", err)
	}
	if target.AbsolutePath != output || target.ConfiguredPath != output || !target.External {
		t.Fatalf("target = %#v, want external absolute target %s", target, output)
	}
}

func TestResolvePluginIndexOutputTargetRejectsRepositoryRelativeTraversal(t *testing.T) {
	root := t.TempDir()
	for _, output := range []string{
		"../plugin-index.json",
		"nested/../../plugin-index.json",
		"nested/../plugin-index.json",
	} {
		t.Run(output, func(t *testing.T) {
			_, err := resolvePluginIndexOutputTarget(root, output, nil)
			if err == nil || !strings.Contains(err.Error(), "clean repository-root-relative path") {
				t.Fatalf("expected clean relative path error, got %v", err)
			}
		})
	}
}

func TestResolvePluginIndexOutputTargetRejectsProtectedRepositoryPaths(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		output string
		want   string
	}{
		{output: ".neko/release.config.json", want: "release configuration"},
		{output: ".neko/release.state.json", want: "release state"},
		{output: ".neko/release.pair-recovery.json", want: "pair-recovery"},
		{output: ".neko/release.migration.json", want: "migration"},
		{output: ".release.neko.json", want: "legacy release configuration"},
		{output: ".release.neko.json.v1.bak", want: "legacy release backup"},
		{output: "plugin/release/manifest.json", want: "plugin manifest"},
		{output: "plugin/ui/manifest.json", want: "plugin manifest"},
		{output: "extensions/audit/manifest.json", want: "plugin manifest"},
		{output: ".git/neko/release/executions/index.json", want: "internal Git state"},
	}
	index := protectedPathTestIndex()
	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			_, err := resolvePluginIndexOutputTarget(root, tt.output, index)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}

	t.Run("absolute path inside repository", func(t *testing.T) {
		_, err := resolvePluginIndexOutputTarget(root, filepath.Join(root, ".neko", "release.state.json"), index)
		if err == nil || !strings.Contains(err.Error(), "release state") {
			t.Fatalf("expected protected absolute repository path error, got %v", err)
		}
	})
}

func TestResolvePluginIndexOutputTargetRejectsUnsafeFilesystemTargets(t *testing.T) {
	t.Run("target is directory", func(t *testing.T) {
		root := t.TempDir()
		output := filepath.Join(root, "dist", "plugin-index.json")
		if err := os.MkdirAll(output, 0700); err != nil {
			t.Fatalf("mkdir output directory: %v", err)
		}

		_, err := resolvePluginIndexOutputTarget(root, "dist/plugin-index.json", nil)
		if err == nil || !strings.Contains(err.Error(), "is a directory") {
			t.Fatalf("expected directory target error, got %v", err)
		}
	})

	t.Run("target is symlink", func(t *testing.T) {
		root := t.TempDir()
		target := filepath.Join(root, "dist", "plugin-index.json")
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatalf("mkdir output parent: %v", err)
		}
		if err := os.Symlink(filepath.Join(root, "real-plugin-index.json"), target); err != nil {
			t.Fatalf("symlink output: %v", err)
		}

		_, err := resolvePluginIndexOutputTarget(root, "dist/plugin-index.json", nil)
		if err == nil || !strings.Contains(err.Error(), "is a symlink") {
			t.Fatalf("expected symlink target error, got %v", err)
		}
	})

	t.Run("repository relative parent resolves outside root", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "dist")); err != nil {
			t.Fatalf("symlink parent: %v", err)
		}

		_, err := resolvePluginIndexOutputTarget(root, "dist/plugin-index.json", nil)
		if err == nil || !strings.Contains(err.Error(), "resolves outside repository root") {
			t.Fatalf("expected parent boundary error, got %v", err)
		}
	})
}

func protectedPathTestIndex() *Index {
	return &Index{
		Plugins: []PluginEntry{
			{Manifest: "plugin/release/manifest.json"},
			{Manifest: "plugin/ui/manifest.json"},
			{Manifest: "extensions/audit/manifest.json"},
		},
	}
}
