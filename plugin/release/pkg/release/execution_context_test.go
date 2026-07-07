package release

//lint:file-ignore SA1019 V1 compatibility tests intentionally use deprecated V1 APIs during migration

import (
	"os"
	"path/filepath"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestBuildReleaseExecutionContextForV1UsesRepositoryRoot(t *testing.T) {
	root := t.TempDir()
	repository := releaseconfig.NormalizeV1Repository(root, &releaseconfig.V1ReleaseConfig{ //nolint:staticcheck // V1 compatibility fixture
		ProjectName:   "neko-cli",
		ReleaseSystem: releaseconfig.V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	})

	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], Patch, true)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	if ctx.RepositoryRoot != root || ctx.UnitRoot != root {
		t.Fatalf("expected V1 roots to be %q, got repo=%q unit=%q", root, ctx.RepositoryRoot, ctx.UnitRoot)
	}
	if ctx.CurrentVersion != "1.2.3" || ctx.NextVersion != "1.2.4" || ctx.Tag != "v1.2.4" {
		t.Fatalf("unexpected version context: %#v", ctx)
	}
	if ctx.SourceFormat != releaseconfig.SourceFormatV1 || ctx.Executor != "goreleaser" || ctx.Delivery != "local" {
		t.Fatalf("unexpected format/executor/delivery: %#v", ctx)
	}
}

func TestBuildReleaseExecutionContextForV2UsesUnitWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	apiRoot := filepath.Join(root, "apps", "api")
	if err := mkdirAll(apiRoot); err != nil {
		t.Fatalf("mkdir api root: %v", err)
	}
	unit := releaseconfig.ReleaseUnit{
		ID:               "api",
		Paths:            []string{"apps/api/**"},
		WorkingDirectory: "apps/api",
		TagPrefix:        "api/v",
		ExecutorType:     "jreleaser",
		Delivery:         "github-actions",
		Workflow:         ".github/workflows/release-api.yml",
		Version:          "0.4.9",
	}
	repository := &releaseconfig.ReleaseRepository{
		RepositoryRoot: root,
		SchemaVersion:  2,
		SourceFormat:   releaseconfig.SourceFormatV2,
		Units:          []releaseconfig.ReleaseUnit{unit},
	}

	ctx, err := BuildReleaseExecutionContext(repository, unit, Minor, true)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	if ctx.UnitRoot != apiRoot {
		t.Fatalf("expected unit root %q, got %q", apiRoot, ctx.UnitRoot)
	}
	if ctx.CurrentVersion != "0.4.9" || ctx.NextVersion != "0.5.0" || ctx.Tag != "api/v0.5.0" {
		t.Fatalf("unexpected version context: %#v", ctx)
	}
	if ctx.DeliveryMode.SupportsLocalExecution {
		t.Fatalf("github-actions delivery must not be locally executable: %#v", ctx.DeliveryMode)
	}
	if ctx.Workflow != ".github/workflows/release-api.yml" {
		t.Fatalf("expected workflow in execution context, got %#v", ctx)
	}
	if !ctx.Capabilities.UpdatesVersionFiles || !ctx.Capabilities.SupportsDryRun {
		t.Fatalf("unexpected jreleaser capabilities: %#v", ctx.Capabilities)
	}
}

func TestBuildReleaseExecutionContextRejectsUnitRootOutsideRepository(t *testing.T) {
	root := t.TempDir()
	unit := releaseconfig.ReleaseUnit{
		ID:               "api",
		WorkingDirectory: "../api",
		TagPrefix:        "api/v",
		ExecutorType:     "goreleaser",
		Delivery:         "local",
		Version:          "1.0.0",
	}
	repository := &releaseconfig.ReleaseRepository{
		RepositoryRoot: root,
		SchemaVersion:  2,
		SourceFormat:   releaseconfig.SourceFormatV2,
		Units:          []releaseconfig.ReleaseUnit{unit},
	}

	if _, err := BuildReleaseExecutionContext(repository, unit, Patch, true); err == nil {
		t.Fatal("expected outside repository error")
	}
}

func TestBuildReleaseExecutionContextRejectsMissingUnitRoot(t *testing.T) {
	root := t.TempDir()
	unit := releaseconfig.ReleaseUnit{
		ID:               "api",
		WorkingDirectory: "api",
		TagPrefix:        "api/v",
		ExecutorType:     "goreleaser",
		Delivery:         "local",
		Version:          "1.0.0",
	}
	repository := &releaseconfig.ReleaseRepository{
		RepositoryRoot: root,
		SchemaVersion:  2,
		SourceFormat:   releaseconfig.SourceFormatV2,
		Units:          []releaseconfig.ReleaseUnit{unit},
	}

	if _, err := BuildReleaseExecutionContext(repository, unit, Patch, true); err == nil {
		t.Fatal("expected missing unit root error")
	}
}

func mkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}
