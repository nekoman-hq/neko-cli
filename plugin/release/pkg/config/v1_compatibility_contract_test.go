//nolint:staticcheck // These tests intentionally pin the deprecated V1 compatibility contract.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV1LoaderCompatibilityFailures(t *testing.T) {
	t.Parallel()

	tests := []struct { //nolint:govet // Logical fixture order keeps identification ahead of file evidence.
		name      string
		content   string
		writeFile bool
		want      string
	}{
		{
			name: "missing",
			want: "configuration not found: no release configuration found. Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config",
		},
		{
			name:      "malformed json",
			content:   `{"project-name":`,
			writeFile: true,
			want:      "configuration parse error:",
		},
		{
			name:      "invalid project type",
			content:   `{"project-type":"desktop","release-system":"goreleaser","version":"1.2.3"}`,
			writeFile: true,
			want:      "invalid configuration: V1ProjectType is invalid in ..release.neko.json",
		},
		{
			name:      "invalid release system",
			content:   `{"project-type":"backend","release-system":"custom","version":"1.2.3"}`,
			writeFile: true,
			want:      "invalid configuration: V1ReleaseSystem is invalid in ..release.neko.json",
		},
		{
			name:      "missing version",
			content:   `{"project-type":"backend","release-system":"goreleaser"}`,
			writeFile: true,
			want:      "invalid configuration: Version is missing in .release.neko.json",
		},
		{
			name:      "invalid semantic version",
			content:   `{"project-type":"backend","release-system":"goreleaser","version":"v1.2.3"}`,
			writeFile: true,
			want:      "invalid configuration: Version is not a valid semantic version (SemVer)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), V1FileName)
			if tt.writeFile {
				if err := os.WriteFile(path, []byte(tt.content), 0600); err != nil {
					t.Fatalf("write V1 fixture: %v", err)
				}
			}
			_, err := V1LoadConfigAt(path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("V1LoadConfigAt error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestV1SaveCompatibilityBytesAndMode(t *testing.T) {
	root := t.TempDir()
	previous, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("get working directory: %v", getwdErr)
	}
	if chdirErr := os.Chdir(root); chdirErr != nil {
		t.Fatalf("enter temp directory: %v", chdirErr)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	cfg := V1ReleaseConfig{
		ProjectName:   "neko-cli",
		ProjectOwner:  "nekoman-hq",
		ProjectType:   V1ProjectTypeBackend,
		ReleaseSystem: V1ReleaseTypeGoReleaser,
		Version:       "1.2.4",
	}
	if saveErr := V1SaveConfig(cfg); saveErr != nil {
		t.Fatalf("V1SaveConfig: %v", saveErr)
	}

	want := "{\n" +
		"  \"project-name\": \"neko-cli\",\n" +
		"  \"project-owner\": \"nekoman-hq\",\n" +
		"  \"project-type\": \"backend\",\n" +
		"  \"release-system\": \"goreleaser\",\n" +
		"  \"version\": \"1.2.4\"\n" +
		"}"
	got, err := os.ReadFile(V1FileName)
	if err != nil {
		t.Fatalf("read saved V1 config: %v", err)
	}
	if string(got) != want {
		t.Fatalf("saved bytes changed:\n got %q\nwant %q", string(got), want)
	}
	info, err := os.Stat(V1FileName)
	if err != nil {
		t.Fatalf("stat saved V1 config: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Fatalf("saved mode = %04o, want 0644", info.Mode().Perm())
	}
}

func TestV1ExplicitRootOperationsIgnoreProcessWorkingDirectory(t *testing.T) {
	repositoryRoot := t.TempDir()
	otherRoot := t.TempDir()

	initial := validV1RootContractConfig()
	if err := V1SaveConfigAt(repositoryRoot, initial); err != nil {
		t.Fatalf("V1SaveConfigAt initial: %v", err)
	}

	previous, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("get working directory: %v", getwdErr)
	}
	if chdirErr := os.Chdir(otherRoot); chdirErr != nil {
		t.Fatalf("enter unrelated directory: %v", chdirErr)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if !V1ConfigExistsAt(repositoryRoot) {
		t.Fatal("V1ConfigExistsAt did not find config through explicit root")
	}
	loaded, err := V1LoadConfigAt(filepath.Join(repositoryRoot, V1FileName))
	if err != nil {
		t.Fatalf("V1LoadConfigAt: %v", err)
	}
	if loaded.Version != initial.Version {
		t.Fatalf("loaded version = %s, want %s", loaded.Version, initial.Version)
	}

	next := initial
	next.Version = "2.0.0"
	if err := V1SaveConfigAt(repositoryRoot, next); err != nil {
		t.Fatalf("V1SaveConfigAt next: %v", err)
	}
	if _, err := os.Stat(filepath.Join(otherRoot, V1FileName)); !os.IsNotExist(err) {
		t.Fatalf("explicit root write touched cwd; stat err=%v", err)
	}
}

func TestV1CwdFacadesUseProcessWorkingDirectory(t *testing.T) {
	repositoryRoot := t.TempDir()
	otherRoot := t.TempDir()

	previous, getwdErr := os.Getwd()
	if getwdErr != nil {
		t.Fatalf("get working directory: %v", getwdErr)
	}
	if chdirErr := os.Chdir(otherRoot); chdirErr != nil {
		t.Fatalf("enter cwd directory: %v", chdirErr)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	cfg := validV1RootContractConfig()
	if err := V1SaveConfig(cfg); err != nil {
		t.Fatalf("V1SaveConfig: %v", err)
	}
	if !V1Exists() {
		t.Fatal("V1Exists did not read process working directory")
	}
	loaded, err := V1LoadConfig()
	if err != nil {
		t.Fatalf("V1LoadConfig: %v", err)
	}
	if loaded.Version != cfg.Version {
		t.Fatalf("loaded version = %s, want %s", loaded.Version, cfg.Version)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, V1FileName)); !os.IsNotExist(err) {
		t.Fatalf("cwd facade unexpectedly touched repository root; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(otherRoot, V1FileName)); err != nil {
		t.Fatalf("cwd facade did not write to cwd: %v", err)
	}
}

func validV1RootContractConfig() V1ReleaseConfig {
	return V1ReleaseConfig{
		ProjectName:   "neko-cli",
		ProjectOwner:  "nekoman-hq",
		ProjectType:   V1ProjectTypeBackend,
		ReleaseSystem: V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	}
}
