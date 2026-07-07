package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2WorkflowStructuralValidation(t *testing.T) {
	tests := []struct {
		name      string
		delivery  DeliveryType
		workflow  string
		wantError string
	}{
		{name: "github actions valid yml", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release-api.yml"},
		{name: "github actions valid yaml", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release-web.yaml"},
		{name: "github actions valid portable filename", delivery: DeliveryGitHubActions, workflow: ".github/workflows/api-v2-release.yml"},
		{name: "github actions missing workflow", delivery: DeliveryGitHubActions, wantError: "requires workflow"},
		{name: "github actions empty workflow", delivery: DeliveryGitHubActions, workflow: " \t", wantError: "must not be empty"},
		{name: "local delivery with workflow", delivery: DeliveryLocal, workflow: ".github/workflows/release-api.yml", wantError: "only valid for github-actions"},
		{name: "filename only", delivery: DeliveryGitHubActions, workflow: "release-api.yml", wantError: "must begin"},
		{name: "absolute path", delivery: DeliveryGitHubActions, workflow: "/.github/workflows/release.yml", wantError: "repository-root-relative"},
		{name: "traversal", delivery: DeliveryGitHubActions, workflow: ".github/workflows/../release.yml", wantError: "traversal"},
		{name: "backslashes", delivery: DeliveryGitHubActions, workflow: `.github\workflows\release.yml`, wantError: "forward slashes"},
		{name: "nested workflow path", delivery: DeliveryGitHubActions, workflow: ".github/workflows/nested/release.yml", wantError: "directly"},
		{name: "uppercase extension", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release.YML", wantError: "lowercase"},
		{name: "query suffix", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release.yml?x=1", wantError: "query"},
		{name: "fragment suffix", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release.yml#fragment", wantError: "fragment"},
		{name: "at suffix", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release.yml@main", wantError: "ref"},
		{name: "invalid filename characters", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release+api.yml", wantError: "must match"},
		{name: "duplicate separators", delivery: DeliveryGitHubActions, workflow: ".github/workflows//release.yml", wantError: "duplicate"},
		{name: "trailing slash", delivery: DeliveryGitHubActions, workflow: ".github/workflows/release.yml/", wantError: "directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateV2WorkflowStructure("api", tt.delivery, tt.workflow)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("expected valid workflow, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func TestV2WorkflowUnknownDeliveryStillFails(t *testing.T) {
	cfg := &V2ReleaseConfig{SchemaVersion: 2, Units: []V2Unit{{
		ID:        "api",
		Paths:     []string{"**"},
		TagPrefix: "v",
		Executor:  V2Executor{Type: ExecutorGoReleaser, Delivery: DeliveryType("ftp")},
	}}}

	if err := ValidateV2ReleaseConfigStructure(cfg); err == nil || !strings.Contains(err.Error(), "unknown delivery") {
		t.Fatalf("expected unknown delivery error, got %v", err)
	}
}

func TestV2WorkflowStructuralValidationDoesNotRequireRepositoryFile(t *testing.T) {
	cfg := &V2ReleaseConfig{SchemaVersion: 2, Units: []V2Unit{{
		ID:        "api",
		Paths:     []string{"api/**"},
		TagPrefix: "api/v",
		Executor: V2Executor{
			Type:     ExecutorJReleaser,
			Delivery: DeliveryGitHubActions,
			Workflow: ".github/workflows/release-api.yml",
		},
	}}}

	if err := ValidateV2ReleaseConfigStructure(cfg); err != nil {
		t.Fatalf("ValidateV2ReleaseConfigStructure: %v", err)
	}
}

func TestV2WorkflowRepositoryValidation(t *testing.T) {
	t.Run("existing valid workflow", func(t *testing.T) {
		root := newV2WorkflowRepository(t, ".github/workflows/release-api.yml")
		if _, err := LoadV2Repository(root); err != nil {
			t.Fatalf("LoadV2Repository: %v", err)
		}
	})

	t.Run("missing workflow", func(t *testing.T) {
		root := t.TempDir()
		writeGitHubActionsV2Files(t, root, ".github/workflows/missing.yml")
		if _, err := LoadV2Repository(root); err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("expected missing workflow error, got %v", err)
		}
	})

	t.Run("directory at workflow path", func(t *testing.T) {
		root := t.TempDir()
		writeGitHubActionsV2Files(t, root, ".github/workflows/release-api.yml")
		mustMkdir(t, filepath.Join(root, ".github", "workflows", "release-api.yml"))
		if _, err := LoadV2Repository(root); err == nil || !strings.Contains(err.Error(), "directory") {
			t.Fatalf("expected directory workflow error, got %v", err)
		}
	})

	t.Run("symlink escaping repository", func(t *testing.T) {
		root := t.TempDir()
		writeGitHubActionsV2Files(t, root, ".github/workflows/release-api.yml")
		outside := filepath.Join(t.TempDir(), "release-api.yml")
		mustWrite(t, outside, "name: outside\n")
		mustMkdir(t, filepath.Join(root, ".github", "workflows"))
		if err := os.Symlink(outside, filepath.Join(root, ".github", "workflows", "release-api.yml")); err != nil {
			t.Skipf("symlink not supported: %v", err)
		}
		if _, err := LoadV2Repository(root); err == nil || !strings.Contains(err.Error(), "outside repository root") {
			t.Fatalf("expected symlink escape error, got %v", err)
		}
	})

	t.Run("local delivery without workflows directory", func(t *testing.T) {
		root := t.TempDir()
		mustMkdir(t, filepath.Join(root, "api"))
		writeV2Files(t, root, validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser", "delivery": "local"}
}`), validV2State(`"api": {"version": "1.0.0"}`))
		if _, err := LoadV2Repository(root); err != nil {
			t.Fatalf("local delivery should not require workflows directory: %v", err)
		}
	})
}

func newV2WorkflowRepository(t *testing.T, workflow string) string {
	t.Helper()
	root := t.TempDir()
	writeGitHubActionsV2Files(t, root, workflow)
	mustWrite(t, filepath.Join(root, filepath.FromSlash(workflow)), "name: release\n")
	return root
}

func writeGitHubActionsV2Files(t *testing.T, root, workflow string) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, "api"))
	writeV2Files(t, root, validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {"type": "jreleaser", "delivery": "github-actions", "workflow": "`+workflow+`"}
}`), validV2State(`"api": {"version": "1.0.0"}`))
}
