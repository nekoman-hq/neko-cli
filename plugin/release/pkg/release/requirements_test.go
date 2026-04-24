package release

import (
	"os"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestValidateRequirementsRequiresGitHubToken(t *testing.T) {
	withWorkingDirectory(t)

	cfg := &releaseconfig.NekoConfig{
		ReleaseSystem: releaseconfig.ReleaseTypeReleaseIt,
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

func TestValidateRequirementsRequiresReleaseSystemConfig(t *testing.T) {
	tests := []struct {
		name          string
		system        releaseconfig.ReleaseSystem
		expectedParts []string
	}{
		{
			name:          "release-it",
			system:        releaseconfig.ReleaseTypeReleaseIt,
			expectedParts: []string{"release-it", releaseItConfigFile},
		},
		{
			name:          "jreleaser",
			system:        releaseconfig.ReleaseTypeJReleaser,
			expectedParts: []string{"jreleaser", jReleaserConfigFile},
		},
		{
			name:          "goreleaser",
			system:        releaseconfig.ReleaseTypeGoReleaser,
			expectedParts: []string{"goreleaser", goReleaserConfigFileYML, goReleaserConfigFileYAML},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			t.Setenv("GITHUB_TOKEN", "test-token")

			cfg := &releaseconfig.NekoConfig{
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
		system     releaseconfig.ReleaseSystem
		configFile string
	}{
		{
			name:       "release-it",
			system:     releaseconfig.ReleaseTypeReleaseIt,
			configFile: releaseItConfigFile,
		},
		{
			name:       "jreleaser",
			system:     releaseconfig.ReleaseTypeJReleaser,
			configFile: jReleaserConfigFile,
		},
		{
			name:       "goreleaser yml",
			system:     releaseconfig.ReleaseTypeGoReleaser,
			configFile: goReleaserConfigFileYML,
		},
		{
			name:       "goreleaser yaml",
			system:     releaseconfig.ReleaseTypeGoReleaser,
			configFile: goReleaserConfigFileYAML,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			t.Setenv("GITHUB_TOKEN", "test-token")

			cfg := &releaseconfig.NekoConfig{
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
