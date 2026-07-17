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
		"workspace.ResolveRepositoryRoot(req.Context.WorkingDir)",
		"return HandleReleaseAt(root, req, releaseType)",
		"return HandleReleaseWithV1ExecutorsAt(root, req, releaseType, executors...)",
		"newFixedV1ReleaseExecutorCatalog(executors...)",
	} {
		if !strings.Contains(handler, required) {
			t.Fatalf("release command handler no longer preserves expected entry-point wiring %q", required)
		}
	}

	start := readCommandBoundarySource(t, "handler.go")
	if !strings.Contains(start, "return newReleaseStartOperationWithV1ExecutorsAt(root, registeredV1ReleaseExecutorCatalog{})") {
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

func TestC1ExecutorRollbackOwnsLegacyRevertReleaseDelegation(t *testing.T) {
	cases := []struct {
		path           string
		revertDelegate string
		rollbackCall   string
	}{
		{
			path:           "tool/goreleaser/goreleaser.go",
			revertDelegate: "func (g *GoReleaser) RevertRelease() error {\n\treturn g.Rollback()\n}",
			rollbackCall:   "return g.rollback.Rollback(g.repositoryRoot, g.CompensationState())",
		},
		{
			path:           "tool/jreleaser/jreleaser.go",
			revertDelegate: "func (j *JReleaser) RevertRelease() error {\n\treturn j.Rollback()\n}",
			rollbackCall:   "return j.rollback.Rollback(j.repositoryRoot, j.CompensationState())",
		},
		{
			path:           "tool/releaseit/release_it.go",
			revertDelegate: "func (r *ReleaseIt) RevertRelease() error {\n\treturn r.Rollback()\n}",
			rollbackCall:   "return r.rollback.Rollback(r.repositoryRoot, r.CompensationState())",
		},
	}
	for _, tt := range cases {
		source := readCommandBoundarySource(t, tt.path)
		if !strings.Contains(source, tt.revertDelegate) {
			t.Fatalf("%s does not keep RevertRelease as direct Rollback delegate", tt.path)
		}
		if !strings.Contains(source, tt.rollbackCall) {
			t.Fatalf("%s Rollback no longer owns the canonical rollback call %q", tt.path, tt.rollbackCall)
		}
	}
}

func TestC1DocsRecommendExplicitReleaseExecutorComposition(t *testing.T) {
	source := readCommandBoundarySource(t, "../../../../docs/ai_context.md")
	for _, required := range []string{
		"goreleaser.NewV1Executor()",
		"jreleaser.NewV1Executor()",
		"releaseit.NewV1Executor()",
		"release.HandleReleaseWithV1Executors(req, release.Patch, v1Executors...)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("AI context no longer recommends explicit V1 executor composition %q", required)
		}
	}
	for _, forbidden := range []string{
		"release.Register(&GoReleaser{})",
		"import _ \"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool\"",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("AI context still recommends deprecated registry composition %q", forbidden)
		}
	}
}

func TestC1DeprecationMarkersMatchCompatibilityPolicy(t *testing.T) {
	required := map[string][]string{
		"service.go": {
			"// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values\n// for release execution, or PlanV1Release for version planning.",
			"// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values\n// instead.",
			"// Deprecated: use PlanV1Release with explicit latest-tag evidence instead.",
		},
		"registry.go": {
			"// Deprecated: use HandleReleaseWithV1Executors with explicit V1Executor values\n// instead.",
			"// Deprecated: use caller-owned V1Executor selection with\n// HandleReleaseWithV1Executors instead.",
		},
		"tool/register.go": {
			"// Deprecated: import concrete executor packages and pass NewV1Executor values\n// to release.HandleReleaseWithV1Executors instead.",
		},
		"execution_context.go": {
			"// Deprecated: use BuildV2ReleaseExecutionContext for V2 release contexts, or\n// PlanV1Release for V1 planning.",
		},
		"version_guard.go": {
			"// Deprecated: use PlanV1Release with explicit latest-tag evidence instead.",
		},
		"../config/v1_loader.go": {
			"// Deprecated: use V1ConfigExistsAt with an explicit repository root instead.",
			"// Deprecated: use V1LoadConfigAt with an explicit path instead.",
			"// Deprecated: use V1SaveConfigAt with an explicit repository root instead.",
		},
		"tool/goreleaser/goreleaser.go": {
			"// Deprecated: use Run with V1ExecutorRequest instead.",
			"// Deprecated: use Rollback instead.",
		},
		"tool/jreleaser/jreleaser.go": {
			"// Deprecated: use Run with V1ExecutorRequest instead.",
			"// Deprecated: use Rollback instead.",
		},
		"tool/releaseit/release_it.go": {
			"// Deprecated: use Run with V1ExecutorRequest instead.",
			"// Deprecated: use Rollback instead.",
		},
	}
	for path, snippets := range required {
		source := readCommandBoundarySource(t, path)
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s is missing C1 deprecation marker:\n%s", path, snippet)
			}
		}
	}

	notDeprecated := map[string][]string{
		"command_handler.go":            {"func HandleRelease(", "func HandleReleaseWithV1Executors("},
		"preflight.go":                  {"func Preflight("},
		"tool.go":                       {"type Tool interface", "type ToolBase struct"},
		"version_guard.go":              {"func EnsureVersionIsValid("},
		"../config/v1_loader.go":        {"func V1ConfigExistsAt(", "func V1LoadConfigAt(", "func V1SaveConfigAt("},
		"tool/goreleaser/goreleaser.go": {"func (g *GoReleaser) Rollback() error"},
		"tool/jreleaser/jreleaser.go":   {"func (j *JReleaser) Rollback() error"},
		"tool/releaseit/release_it.go":  {"func (r *ReleaseIt) Rollback() error"},
	}
	for path, signatures := range notDeprecated {
		source := readCommandBoundarySource(t, path)
		for _, signature := range signatures {
			block := c1DeclarationBlock(t, source, signature)
			if strings.Contains(block, "Deprecated:") {
				t.Fatalf("%s keeps/defer surface %q was marked deprecated", path, signature)
			}
		}
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

func c1DeclarationBlock(t *testing.T, source string, signature string) string {
	t.Helper()
	lines := strings.Split(source, "\n")
	index := -1
	for i, line := range lines {
		if strings.Contains(line, signature) {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatalf("source is missing declaration %q", signature)
	}

	start := index
	for start > 0 {
		previous := strings.TrimSpace(lines[start-1])
		if !strings.HasPrefix(previous, "//") {
			break
		}
		start--
	}

	end := index + 1
	for end < len(lines) {
		line := strings.TrimSpace(lines[end])
		if line == "" && c1NextNonCommentLineStartsDeclaration(lines[end+1:]) {
			break
		}
		if strings.HasPrefix(line, "func ") || strings.HasPrefix(line, "type ") {
			break
		}
		end++
	}
	return strings.Join(lines[start:end], "\n")
}

func c1NextNonCommentLineStartsDeclaration(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		return strings.HasPrefix(trimmed, "func ") || strings.HasPrefix(trimmed, "type ")
	}
	return false
}
