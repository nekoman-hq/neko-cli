package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestHandleValidateV2Show(t *testing.T) {
	withWorkingDirectory(t)
	writeV2(t, `{"schemaVersion":2,"units":[{
  "id":"default",
  "paths":["**"],
  "tagPrefix":"v",
  "executor":{"type":"goreleaser"}
}]}`, `{"schemaVersion":2,"units":{"default":{"version":"2.2.4"}}}`)

	resp, err := HandleValidate(plugin.Request{Flags: map[string]any{"show": true}})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected success, got %#v", resp.Error)
	}
	if !itemsContain(resp.Data["items"], "delivery=local") {
		t.Fatalf("expected normalized delivery in show output, got %#v", resp.Data["items"])
	}
}

func TestHandleValidateV1StillUsesLegacyConfig(t *testing.T) {
	withWorkingDirectory(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	mustWrite(t, config.FileName, `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`)
	mustWrite(t, ".goreleaser.yml", "{}")

	resp, err := HandleValidate(plugin.Request{})
	if err != nil {
		t.Fatalf("HandleValidate: %v", err)
	}
	if resp.Status != "success" {
		t.Fatalf("expected V1 success, got %#v", resp.Error)
	}
}

func writeV2(t *testing.T, cfg, state string) {
	t.Helper()
	mustWrite(t, config.V2ConfigPath("."), cfg)
	mustWrite(t, config.V2StatePath("."), state)
}

func withWorkingDirectory(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func itemsContain(items any, expected string) bool {
	rows, ok := items.([]map[string]any)
	if !ok {
		return false
	}
	for _, row := range rows {
		for _, value := range row {
			if value == expected {
				return true
			}
			if s, ok := value.(string); ok && strings.Contains(s, expected) {
				return true
			}
		}
	}
	return false
}
