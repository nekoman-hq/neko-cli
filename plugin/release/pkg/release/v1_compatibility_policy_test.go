package release

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestC1ReleaseEntryPointsKeepProductionOffMutableRegistry(t *testing.T) {
	handler := readCommandBoundarySource(t, "command_handler.go")
	for _, required := range []string{
		"return handleReleaseWithStarter(req, releaseType, newReleaseStartOperation())",
		"newFixedV1ReleaseExecutorCatalog(executors...)",
	} {
		if !strings.Contains(handler, required) {
			t.Fatalf("release command handler no longer preserves expected entry-point wiring %q", required)
		}
	}

	start := readCommandBoundarySource(t, "handler.go")
	if !strings.Contains(start, "return newReleaseStartOperationWithV1Executors(registeredV1ReleaseExecutorCatalog{})") {
		t.Fatal("registry-backed release startup is no longer confined to the compatibility HandleRelease path")
	}

	mainSource := readCommandBoundarySource(t, "../../main.go")
	for _, required := range []string{
		"goreleaser.NewV1Executor()",
		"jreleaser.NewV1Executor()",
		"releaseit.NewV1Executor()",
		"release.HandleReleaseWithV1Executors(req, release.Patch, v1Executors...)",
		"release.HandleReleaseWithV1Executors(req, release.Minor, v1Executors...)",
		"release.HandleReleaseWithV1Executors(req, release.Major, v1Executors...)",
	} {
		if !strings.Contains(mainSource, required) {
			t.Fatalf("production main no longer uses explicit fixed V1 composition %q", required)
		}
	}
	for _, forbidden := range []string{
		"release.HandleRelease(req",
		"release.Register(",
		"_ \"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool\"",
	} {
		if strings.Contains(mainSource, forbidden) {
			t.Fatalf("production main uses mutable registry compatibility surface %q", forbidden)
		}
	}
}

func TestC1RegistryAndVersionGlobalsRemainCompatibilityOnly(t *testing.T) {
	assertC1ProductionReferences(t, "Register(", []string{
		"registry.go",
		"tool/register.go",
	})
	assertC1ProductionReferences(t, "func Get(", []string{
		"registry.go",
	})
	assertC1ProductionReferences(t, "Get(name)", []string{
		"v1_release_adapters.go",
	})
	assertC1ProductionReferences(t, "refreshVersionTags", []string{
		"version_guard.go",
		"v1_release_adapters.go",
	})
	assertC1ProductionReferences(t, "latestVersionTag", []string{
		"version_guard.go",
		"v1_release_adapters.go",
	})
}

func TestC1V1ConfigCurrentDirectoryFacadesRemainDirectDelegates(t *testing.T) {
	source := readCommandBoundarySource(t, "../config/v1_loader.go")
	for _, required := range []string{
		"func V1Exists() bool {\n\treturn V1ConfigExistsAt(\".\")\n}",
		"func V1LoadConfig() (*V1ReleaseConfig, error) {\n\treturn V1LoadConfigAt(V1FileName)\n}",
		"func V1SaveConfig(config V1ReleaseConfig) error {\n\treturn V1SaveConfigAt(\".\", config)\n}",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("V1 current-directory facade is no longer a direct delegate:\n%s", required)
		}
	}
	for _, forbidden := range []string{"os.Chdir", "exec.Command"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("V1 config facade gained process or command side effect %q", forbidden)
		}
	}
}

func TestC1ExecutorRollbackStillDelegatesThroughLegacyRevertRelease(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{
			path: "tool/goreleaser/goreleaser.go",
			want: "func (g *GoReleaser) Rollback() error { return g.RevertRelease() }",
		},
		{
			path: "tool/jreleaser/jreleaser.go",
			want: "func (j *JReleaser) Rollback() error { return j.RevertRelease() }",
		},
		{
			path: "tool/releaseit/release_it.go",
			want: "func (r *ReleaseIt) Rollback() error { return r.RevertRelease() }",
		},
	}
	for _, tt := range cases {
		source := readCommandBoundarySource(t, tt.path)
		if !strings.Contains(source, tt.want) {
			t.Fatalf("%s no longer has characterized legacy rollback delegate %q", tt.path, tt.want)
		}
	}
}

func TestC1DocsStillCarryRegistryExamplePendingPolicyUpdate(t *testing.T) {
	source := readCommandBoundarySource(t, "../../../../docs/ai_context.md")
	if !strings.Contains(source, "release.Register(&GoReleaser{})") {
		t.Fatal("AI context no longer records the pre-C1 registry example; update this test with the docs migration")
	}
}

func assertC1ProductionReferences(t *testing.T, needle string, want []string) {
	t.Helper()
	got := c1ProductionReferences(t, needle)
	slices.Sort(got)
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("production references to %q = %v, want %v", needle, got, want)
	}
}

func c1ProductionReferences(t *testing.T, needle string) []string {
	t.Helper()
	var references []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), needle) {
			references = append(references, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk production release sources: %v", err)
	}
	return references
}
