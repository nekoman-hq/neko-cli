package contextvalidation

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseContextValidationHasOnlyReadOnlyLocalDependencies(t *testing.T) {
	useCaseBytes, err := os.ReadFile("release_context_validation.go")
	if err != nil {
		t.Fatalf("read use case: %v", err)
	}
	useCase := string(useCaseBytes)
	for _, forbidden := range []string{
		"GitHubActionsDispatchToken",
		"GitHubActionsDispatcher",
		"http.Client",
		"os.Getenv",
		"os.WriteFile",
		"AtomicWrite",
		"JournalStore",
		"MaterializationTransaction",
		"StateTransaction",
		"GitReleaseCoordinator",
		"ReleaseRunner",
		"plugin.Response",
	} {
		if strings.Contains(useCase, forbidden) {
			t.Fatalf("release context use case must not access %q", forbidden)
		}
	}
	for _, required := range []string{
		"sources releaseContextSourceReader",
		"git     releaseContextGitReader",
		"ResolveReleaseUnit(repository, request.UnitID",
		"NewTagSpec(unit.TagPrefix)",
	} {
		if !strings.Contains(useCase, required) {
			t.Fatalf("release context use case is missing boundary %q", required)
		}
	}

	sourceBytes, err := os.ReadFile("release_context_source.go")
	if err != nil {
		t.Fatalf("read source adapter: %v", err)
	}
	source := string(sourceBytes)
	for _, forbidden := range []string{
		"LoadReleaseRepository(",
		"V1LoadConfig",
		"V2ReleasePairPersister",
		"recoverUnresolvedPair",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("V2-only source reader must not access %q", forbidden)
		}
	}
	for _, required := range []string{
		"LoadV2Config(",
		"LoadV2State(",
		"ValidateV2(",
		"ValidateV2PairRecoveryReadiness(",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("V2-only source reader is missing %q", required)
		}
	}

	gitBytes, err := os.ReadFile("release_context_git.go")
	if err != nil {
		t.Fatalf("read Git adapter: %v", err)
	}
	gitSource := string(gitBytes)
	for _, forbidden := range []string{"\"fetch\"", "\"checkout\"", "\"reset\"", "\"tag\", tag", "\"push\"", "\"commit\""} {
		if strings.Contains(gitSource, forbidden) {
			t.Fatalf("release context Git adapter must not contain mutating command %q", forbidden)
		}
	}
	for _, required := range []string{"--show-object-format", "cat-file", "HEAD^{commit}", "show-ref", "^{commit}"} {
		if !strings.Contains(gitSource, required) {
			t.Fatalf("release context Git adapter is missing read capability %q", required)
		}
	}
}

func TestContextValidationCapabilityDoesNotImportRootReleaseOrChainHandlers(t *testing.T) {
	for _, path := range []string{
		"boundary.go", "git_runner.go", "release_context_command.go", "release_context_git.go",
		"release_context_response.go", "release_context_source.go", "release_context_validation.go",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(data)
		for _, forbidden := range []string{
			"plugin/release/pkg/release", "net/http", "GITHUB_TOKEN", "TokenResolver",
			"os.WriteFile", "os.Mkdir", "HandleUnits", "HandleDoctor", "HandleGitHubWorkflowInit",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited dependency %q", path, forbidden)
			}
		}
	}
}
