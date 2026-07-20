package doctor

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	rootReleaseImportPath = "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	dispatchImportPath    = "github.com/nekoman-hq/neko-cli/plugin/release/internal/githubdispatch"
)

func TestDoctorCapabilityDoesNotImportRootReleaseOrDispatchMutation(t *testing.T) {
	for _, path := range doctorProductionFiles(t) {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), path, source, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, specification := range parsed.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", path, err)
			}
			if importPath == rootReleaseImportPath || importPath == dispatchImportPath {
				t.Errorf("%s imports prohibited capability %s", path, importPath)
			}
		}
		text := string(source)
		for _, forbidden := range []string{
			"http.MethodPost", "http.MethodPut", "http.MethodPatch", "http.MethodDelete",
			"GitHubActionsDispatchClient", "GitHubActionsDispatcher", "DispatchJournal",
			"ReleaseExecutionJournal", "os.WriteFile", "os.Mkdir", "os.Chdir", "exec.Command",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s contains prohibited mutation or lifecycle capability %q", path, forbidden)
			}
		}
	}
}

func TestDoctorRemoteTransportIsOneBoundedGETBoundary(t *testing.T) {
	client, err := os.ReadFile("integration_doctor_github_read_client.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(client)
	if strings.Count(text, "http.MethodGet") != 1 || strings.Count(text, "httpClient.Do(request)") != 1 {
		t.Fatal("Doctor GitHub reader must retain one bounded GET transport boundary")
	}
	inspection, err := os.ReadFile("integration_doctor_inspection.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(inspection), "if request.VerifyRemote && useCase.remote != nil") {
		t.Fatal("Doctor remote inspection is not guarded by the explicit request")
	}
}

func doctorProductionFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			paths = append(paths, entry.Name())
		}
	}
	return paths
}
