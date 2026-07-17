package pluginindex

import (
	"path/filepath"
	"testing"
)

func TestResolvePluginIndexOutputTargetUsesRepositoryRootForRelativePaths(t *testing.T) {
	root := t.TempDir()
	target, err := resolvePluginIndexOutputTarget(root, "nested/plugin-index.json")
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
	target, err := resolvePluginIndexOutputTarget(root, output)
	if err != nil {
		t.Fatalf("resolvePluginIndexOutputTarget: %v", err)
	}
	if target.AbsolutePath != output || target.ConfiguredPath != output || !target.External {
		t.Fatalf("target = %#v, want external absolute target %s", target, output)
	}
}
