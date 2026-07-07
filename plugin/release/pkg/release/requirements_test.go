//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

//lint:file-ignore SA1019 V1 compatibility release paths intentionally use deprecated V1 APIs during migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestValidateRequirementsRequiresGitHubToken(t *testing.T) {
	withWorkingDirectory(t)

	cfg := &releaseconfig.V1ReleaseConfig{
		ReleaseSystem: releaseconfig.V1ReleaseTypeReleaseIt,
	}

	if err := os.WriteFile(releaseItConfigFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("write %s: %v", releaseItConfigFile, err)
	}

	t.Setenv("GITHUB_TOKEN", "")

	err := ValidateRequirements(cfg)
	if err == nil {
		t.Fatal("expected missing token error")
	}

	if !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("expected error to mention GITHUB_TOKEN, got %q", err.Error())
	}
}

func TestValidateRequirementsForContextUsesUnitRoot(t *testing.T) {
	repositoryRoot := t.TempDir()
	unitRoot := filepath.Join(repositoryRoot, "api")
	if err := os.MkdirAll(unitRoot, 0755); err != nil {
		t.Fatalf("mkdir unit root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(unitRoot, goReleaserConfigFileYML), []byte("{}"), 0644); err != nil {
		t.Fatalf("write unit config: %v", err)
	}
	otherRoot := t.TempDir()
	cwd, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("getwd: %v", getwdErr)
	}
	if chdirErr := os.Chdir(otherRoot); chdirErr != nil {
		t.Fatalf("chdir other root: %v", chdirErr)
	}
	t.Cleanup(func() {
		if restoreErr := os.Chdir(cwd); restoreErr != nil {
			t.Fatalf("restore cwd %s: %v", cwd, restoreErr)
		}
	})
	t.Setenv("GITHUB_TOKEN", "test-token")

	ctx := &ReleaseExecutionContext{
		Executor: "goreleaser",
		UnitRoot: unitRoot,
	}

	if err := ValidateRequirementsForContext(ctx); err != nil {
		t.Fatalf("expected requirements to pass using UnitRoot, got %v", err)
	}
}

func TestValidateRequirementsForContextDoesNotUseRepositoryRootFallback(t *testing.T) {
	repositoryRoot := t.TempDir()
	unitRoot := filepath.Join(repositoryRoot, "api")
	if err := os.MkdirAll(unitRoot, 0755); err != nil {
		t.Fatalf("mkdir unit root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repositoryRoot, goReleaserConfigFileYML), []byte("{}"), 0644); err != nil {
		t.Fatalf("write repository config: %v", err)
	}
	t.Setenv("GITHUB_TOKEN", "test-token")

	ctx := &ReleaseExecutionContext{
		Executor: "goreleaser",
		UnitRoot: unitRoot,
	}

	err := ValidateRequirementsForContext(ctx)
	if err == nil {
		t.Fatal("expected missing unit-local config error")
	}
	if !strings.Contains(err.Error(), goReleaserConfigFileYML) {
		t.Fatalf("expected error to mention goreleaser config, got %q", err.Error())
	}
}

func TestValidateRequirementsRequiresReleaseSystemConfig(t *testing.T) {
	tests := []struct {
		name          string
		system        releaseconfig.V1ReleaseSystem
		expectedParts []string
	}{
		{
			name:          "release-it",
			system:        releaseconfig.V1ReleaseTypeReleaseIt,
			expectedParts: []string{"release-it", releaseItConfigFile},
		},
		{
			name:          "jreleaser",
			system:        releaseconfig.V1ReleaseTypeJReleaser,
			expectedParts: []string{"jreleaser", jReleaserConfigFile},
		},
		{
			name:          "goreleaser",
			system:        releaseconfig.V1ReleaseTypeGoReleaser,
			expectedParts: []string{"goreleaser", goReleaserConfigFileYML, goReleaserConfigFileYAML},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			t.Setenv("GITHUB_TOKEN", "test-token")

			cfg := &releaseconfig.V1ReleaseConfig{
				ReleaseSystem: tt.system,
			}

			err := ValidateRequirements(cfg)
			if err == nil {
				t.Fatal("expected missing config error")
			}

			for _, part := range tt.expectedParts {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("expected error to contain %q, got %q", part, err.Error())
				}
			}
		})
	}
}

func TestValidateRequirementsAcceptsExistingConfig(t *testing.T) {
	tests := []struct {
		name       string
		system     releaseconfig.V1ReleaseSystem
		configFile string
	}{
		{
			name:       "release-it",
			system:     releaseconfig.V1ReleaseTypeReleaseIt,
			configFile: releaseItConfigFile,
		},
		{
			name:       "jreleaser",
			system:     releaseconfig.V1ReleaseTypeJReleaser,
			configFile: jReleaserConfigFile,
		},
		{
			name:       "goreleaser yml",
			system:     releaseconfig.V1ReleaseTypeGoReleaser,
			configFile: goReleaserConfigFileYML,
		},
		{
			name:       "goreleaser yaml",
			system:     releaseconfig.V1ReleaseTypeGoReleaser,
			configFile: goReleaserConfigFileYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			t.Setenv("GITHUB_TOKEN", "test-token")

			cfg := &releaseconfig.V1ReleaseConfig{
				ReleaseSystem: tt.system,
			}

			if err := os.WriteFile(tt.configFile, []byte("{}"), 0644); err != nil {
				t.Fatalf("write %s: %v", tt.configFile, err)
			}

			if err := ValidateRequirements(cfg); err != nil {
				t.Fatalf("expected requirements to pass, got %v", err)
			}
		})
	}
}

func withWorkingDirectory(t *testing.T) {
	t.Helper()

	cwd, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("getwd: %v", getwdErr)
	}

	tempDir := t.TempDir()
	if chdirErr := os.Chdir(tempDir); chdirErr != nil {
		t.Fatalf("chdir %s: %v", tempDir, chdirErr)
	}

	t.Cleanup(func() {
		if restoreErr := os.Chdir(cwd); restoreErr != nil {
			t.Fatalf("restore cwd %s: %v", cwd, restoreErr)
		}
	})
}
