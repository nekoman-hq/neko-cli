package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadV2RepositoryValidSingleUnit(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "app"))
	writeV2Files(t, root, validV2Config(`{
  "id": "default",
  "displayName": "My Project",
  "paths": ["**"],
  "workingDirectory": "app",
  "tagPrefix": "v",
  "executor": {
    "type": "goreleaser",
    "delivery": "local"
  }
}`), validV2State(`"default": {"version": "2.2.4"}`))

	repo, err := LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	if repo.SourceFormat != SourceFormatV2 || len(repo.Units) != 1 {
		t.Fatalf("unexpected repository: %#v", repo)
	}
	unit := repo.Units[0]
	if unit.ID != "default" || unit.Version != "2.2.4" || unit.WorkingDirectory != "app" || unit.Delivery != "local" {
		t.Fatalf("unexpected normalized unit: %#v", unit)
	}
}

func TestLoadV2RepositoryValidMultiUnitWithDefaults(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "api"))
	mustMkdir(t, filepath.Join(root, "web"))
	writeV2Files(t, root, validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser"}
}, {
  "id": "web",
  "paths": ["web/**"],
  "workingDirectory": "web",
  "tagPrefix": "web/v",
  "executor": {"type": "release-it", "delivery": "github-actions", "workflow": ".github/workflows/release-web.yml"}
}`), validV2State(`"api": {"version": "1.0.0"}, "web": {"version": "2.0.0"}`))
	mustWrite(t, filepath.Join(root, ".github", "workflows", "release-web.yml"), "name: release web\n")

	repo, err := LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	if len(repo.Units) != 2 {
		t.Fatalf("expected 2 units, got %#v", repo.Units)
	}
	if repo.Units[0].Delivery != "local" {
		t.Fatalf("expected default local delivery, got %s", repo.Units[0].Delivery)
	}
	if repo.Units[1].Delivery != "github-actions" {
		t.Fatalf("expected github-actions delivery, got %s", repo.Units[1].Delivery)
	}
	if repo.Units[1].Workflow != ".github/workflows/release-web.yml" {
		t.Fatalf("expected normalized workflow, got %#v", repo.Units[1])
	}
}

func TestNormalizeV1Repository(t *testing.T) {
	cfg := &V1ReleaseConfig{
		ProjectName:   "neko-cli",
		ProjectOwner:  "nekoman-hq",
		ProjectType:   V1ProjectTypeBackend,
		ReleaseSystem: V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	}

	repo := NormalizeV1Repository("/repo", cfg)
	if repo.SourceFormat != SourceFormatV1 || repo.SchemaVersion != 1 || repo.Legacy != cfg {
		t.Fatalf("unexpected V1 repository: %#v", repo)
	}
	unit := repo.Units[0]
	if unit.ID != "default" || unit.Paths[0] != "**" || unit.TagPrefix != "v" || unit.Version != "1.2.3" {
		t.Fatalf("unexpected V1 normalized unit: %#v", unit)
	}
	if unit.Workflow != "" {
		t.Fatalf("V1 normalized unit must not carry workflow: %#v", unit)
	}
}

func TestLoadReleaseRepositoryRejectsRootV1V2Conflict(t *testing.T) {
	root := t.TempDir()
	writeV1Config(t, root)
	writeV2Files(t, root, validV2Config(`{
  "id": "default",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`), validV2State(`"default": {"version": "1.0.0"}`))

	_, err := LoadReleaseRepository(root)
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}

func TestLoadV2RepositoryValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		state     string
		wantError string
	}{
		{
			name:      "unknown config field",
			config:    `{"schemaVersion":2,"unknown":true,"units":[]}`,
			state:     validV2State(``),
			wantError: "unknown field",
		},
		{
			name:      "wrong config schema",
			config:    `{"schemaVersion":1,"units":[]}`,
			state:     validV2State(``),
			wantError: "schemaVersion",
		},
		{
			name: "duplicate unit id",
			config: validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser"}
}, {
  "id": "api",
  "paths": ["web/**"],
  "tagPrefix": "web/v",
  "executor": {"type": "release-it"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "more than once",
		},
		{
			name: "bad unit id",
			config: validV2Config(`{
  "id": "Bad/ID",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"Bad/ID": {"version": "1.0.0"}`),
			wantError: "invalid",
		},
		{
			name: "overlapping tag prefix",
			config: validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "tagPrefix": "api/v",
  "executor": {"type": "goreleaser"}
}, {
  "id": "web",
  "paths": ["web/**"],
  "tagPrefix": "api/v1",
  "executor": {"type": "release-it"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}, "web": {"version": "1.0.0"}`),
			wantError: "overlaps",
		},
		{
			name: "working directory leaves repo",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "workingDirectory": "../api",
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "leaves the repository",
		},
		{
			name: "path leaves repo",
			config: validV2Config(`{
  "id": "api",
  "paths": ["../**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "path pattern",
		},
		{
			name: "unknown executor",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "custom"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "unknown executor",
		},
		{
			name: "unknown delivery",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser", "delivery": "ftp"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}`),
			wantError: "unknown delivery",
		},
		{
			name: "missing state entry",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(``),
			wantError: "missing unit",
		},
		{
			name: "unknown state unit",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "1.0.0"}, "web": {"version": "1.0.0"}`),
			wantError: "unknown unit",
		},
		{
			name: "bad semver",
			config: validV2Config(`{
  "id": "api",
  "paths": ["**"],
  "tagPrefix": "v",
  "executor": {"type": "goreleaser"}
}`),
			state:     validV2State(`"api": {"version": "not-semver"}`),
			wantError: "SemVer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeV2Files(t, root, tt.config, tt.state)

			_, err := LoadV2Repository(root)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}

func validV2Config(units string) string {
	return `{"schemaVersion":2,"units":[` + units + `]}`
}

func validV2State(units string) string {
	return `{"schemaVersion":2,"units":{` + units + `}}`
}

func writeV2Files(t *testing.T, root, cfg, state string) {
	t.Helper()
	mustMkdir(t, filepath.Join(root, V2Directory))
	mustWrite(t, V2ConfigPath(root), cfg)
	mustWrite(t, V2StatePath(root), state)
}

func writeV1Config(t *testing.T, root string) {
	t.Helper()
	mustWrite(t, filepath.Join(root, V1FileName), `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`)
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
