package release

import (
	"go/ast"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestCommandHandlersRemainPresentationBoundaries(t *testing.T) {
	source := readCommandBoundarySource(t, "command_handler.go")
	for _, forbidden := range []string{
		"LoadReleaseRepository",
		"ResolveReleaseUnit",
		"BuildReleaseExecutionContext",
		"NewGitHubActionsReleaseRunner",
		"ReleaseExecutionJournalStore",
		"resumeJournal",
		"GITHUB_TOKEN",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("command_handler.go contains application responsibility %q", forbidden)
		}
	}
}

func TestCommandApplicationOperationsDoNotConstructPluginResponses(t *testing.T) {
	for _, path := range []string{"handler.go", "resume.go", "github_actions_release_runner.go", "github_actions_release_use_case.go"} {
		source := readCommandBoundarySource(t, path)
		if strings.Contains(source, "github.com/nekoman-hq/neko-cli/pkg/plugin") || strings.Contains(source, "plugin.Response") {
			t.Fatalf("%s depends on plugin response presentation", path)
		}
	}
}

func TestGitHubActionsReleaseRunnerRemainsFacade(t *testing.T) {
	source := readCommandBoundarySource(t, "github_actions_release_runner.go")
	for _, forbidden := range []string{
		"BeginPending",
		"ConfirmPhase",
		"BuildReleaseExecutionJournal",
		"NewReleaseExecutionJournalStore",
		"ResolveVersionMaterializer",
		"PushCommit",
		"NewGitHubActionsDispatcher",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("github_actions_release_runner.go contains extracted responsibility %q", forbidden)
		}
	}
}

func TestResumeArchitectureHasNoUniversalPlaybookOrBooleanModes(t *testing.T) {
	for _, path := range []string{"resume.go", "resume_policy.go", "resume_operations.go"} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"func resumeJournal",
			"loadOnly bool",
			"pushed bool",
			"ResumeManager",
			"ResumeService",
			"RecoveryCoordinator",
			"TransitionEngine",
			"WorkflowPipeline",
			"PhaseProcessor",
			"DependencyBag",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited resume architecture %q", path, forbidden)
			}
		}
	}
}

func TestResumeOperationsReuseFocusedActiveReleaseCapabilities(t *testing.T) {
	source := readCommandBoundarySource(t, "resume.go")
	for _, capability := range []string{
		"createGitHubActionsReleaseTag",
		"prepareGitHubActionsReleaseDispatch",
		"pushGitHubActionsReleaseCommit",
		"pushGitHubActionsReleaseTag",
		"dispatchGitHubActionsReleaseWorkflow",
		"confirmGitHubActionsReleaseHandoff",
	} {
		if !strings.Contains(source, capability) {
			t.Fatalf("resume composition does not reuse active release capability %q", capability)
		}
	}
}

func TestActiveV2BuildersAndRecoveryDoNotConstructGitInfrastructure(t *testing.T) {
	for _, path := range []string{"release_dispatch_request.go", "release_execution_recovery.go"} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{"NewGitReleaseCoordinator", "exec.Command"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s constructs Git infrastructure through %q", path, forbidden)
			}
		}
	}
}

func TestActiveV2TokenBoundaryHasNoStaticResolverAdapter(t *testing.T) {
	source := readCommandBoundarySource(t, "github_actions_release_operations.go")
	if strings.Contains(source, "staticGitHubActionsDispatchTokenResolver") {
		t.Fatal("active V2 dispatch wraps its token in a second resolver boundary")
	}
}

func TestActiveV2ProgressDoesNotImportGlobalTerminalLogger(t *testing.T) {
	for _, path := range []string{
		"handler.go",
		"github_actions_release_runner.go",
		"github_actions_release_use_case.go",
		"github_actions_release_operations.go",
		"github_actions_dispatcher.go",
		"git_release_preflight.go",
		"git_release_coordinator.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"github.com/nekoman-hq/neko-cli/pkg/log",
			"log.PluginPrint",
			"log.PluginV",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s still depends on global terminal logger through %q", path, forbidden)
			}
		}
	}
}

func TestTerminalRenderingIsOwnedByAdapters(t *testing.T) {
	for _, path := range []string{"release_progress_terminal.go", "git_release_diagnostics_terminal.go"} {
		source := readCommandBoundarySource(t, path)
		if !strings.Contains(source, "github.com/nekoman-hq/neko-cli/pkg/log") {
			t.Fatalf("%s is expected to own terminal logger access", path)
		}
	}
}

func TestProgressBoundaryDoesNotConstructResponsesOrEventInfrastructure(t *testing.T) {
	for _, path := range []string{"release_progress.go", "release_progress_terminal.go"} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"plugin.Response",
			"github.com/nekoman-hq/neko-cli/pkg/plugin",
			"ProgressManager",
			"ProgressCoordinator",
			"ProgressService",
			"EventBus",
			"Subscribe(",
			"Publish(",
			"[]ReleaseProgress{",
			"chan ReleaseProgress",
			"func(",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited progress architecture %q", path, forbidden)
			}
		}
	}
}

func TestReleasePlanInspectionUsesCanonicalPlanningBoundaries(t *testing.T) {
	source := readCommandBoundarySource(t, "release_plan_inspection.go")
	for _, required := range []string{
		"selectReleaseApplicationPath",
		"ResolveReleaseUnit",
		"PlanV1Release",
		"BuildV2ReleaseExecutionContext",
		"planV2ReleaseFacts",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("release plan inspection does not use canonical planning boundary %q", required)
		}
	}
}

func TestReleasePlanInspectionHasNoMutationTokenRemoteOrJournalDependencies(t *testing.T) {
	source := readCommandBoundarySource(t, "release_plan_inspection.go")
	for _, forbidden := range []string{
		"GitHubActionsDispatchToken",
		"TokenResolver",
		"EnvironmentGitHubActionsDispatchTokenResolver",
		"GITHUB_TOKEN",
		"GitHubActionsDispatcher",
		"GitHubActionsDispatchClient",
		"NewGitHubActionsReleaseRunner",
		"NewReleaseExecutionJournalStore",
		"NewDispatchJournalStore",
		"ReleaseExecutionJournalStore",
		"DispatchJournalStore",
		"NewGitReleaseCoordinator",
		"MaterializationTransaction",
		"StateTransaction",
		"V1Compensation",
		"exec.Command",
		"os.WriteFile",
		"os.MkdirAll",
		"plugin.Response",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("release_plan_inspection.go contains prohibited dependency %q", forbidden)
		}
	}
}

func TestReleasePlanInspectionAvoidsGenericInspectionArchitecture(t *testing.T) {
	source := readCommandBoundarySource(t, "release_plan_inspection.go")
	for _, forbidden := range []string{
		"InspectionManager",
		"PlanningManager",
		"PipelineManager",
		"WorkflowPipeline",
		"TransitionEngine",
		"DependencyBag",
		"ServiceLocator",
		"planOnly",
		"inspectionMode",
		"executeNothing",
		"fakeDryRun",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("release plan inspection contains prohibited architecture %q", forbidden)
		}
	}
}

func TestIntegrationDoctorApplicationHasNoMutationTokenNetworkGitOrStoreDependencies(t *testing.T) {
	for _, path := range []string{
		"integration_doctor_consumer.go",
		"integration_doctor_credentials.go",
		"integration_doctor_goreleaser.go",
		"integration_doctor_inspection.go",
		"integration_doctor_installation.go",
		"integration_doctor_limitations.go",
		"integration_doctor_publication.go",
		"integration_doctor_repository_files.go",
		"integration_doctor_repository_identity.go",
		"integration_doctor_source.go",
		"integration_doctor_permissions.go",
		"integration_doctor_verification.go",
		"integration_doctor_workflow_inspection.go",
		"integration_doctor_workflow_reader.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"TokenResolver",
			"GitHubActionsDispatcher",
			"GitHubActionsDispatchClient",
			"net/http",
			"os/exec",
			"exec.Command",
			"NewGitReleaseCoordinator",
			"os.WriteFile",
			"os.Mkdir",
			"os.Chdir",
			"os.Setenv",
			"os.Getenv",
			"os.LookupEnv",
			"ReleaseExecutionJournalStore",
			"DispatchJournalStore",
			"EvidenceWriter",
			"atomicGitHubWorkflowOutputCreator",
			"plugin.Response",
			"internal/terminal",
			"golang.org/x/term",
			"StyleRole",
			"\\x1b",
			"\\033",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited doctor dependency %q", path, forbidden)
			}
		}
	}
}

func TestIntegrationDoctorKeepsTypedCommandAndResponseBoundaries(t *testing.T) {
	command := readCommandBoundarySource(t, "integration_doctor_command.go")
	for _, required := range []string{"parseIntegrationDoctorRequest", "handler.inspector.Inspect", "mapIntegrationDoctorResult"} {
		if !strings.Contains(command, required) {
			t.Fatalf("doctor command boundary omits %q", required)
		}
	}
	for _, forbidden := range []string{"LoadV2Config", "yaml.Unmarshal", "inspectIntegrationDoctorWorkflow("} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("doctor command boundary contains inspection responsibility %q", forbidden)
		}
	}
	response := readCommandBoundarySource(t, "integration_doctor_response.go")
	if !strings.Contains(response, "plugin.Response") {
		t.Fatal("doctor response mapper does not own the plugin response boundary")
	}
}

func TestIntegrationDoctorUsesCanonicalGoReleaserFacts(t *testing.T) {
	const canonicalImport = "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/goreleaser"
	requiredCallNames := []string{"ParseConfig", "ClassifyArguments", "VerifyArtifactContract"}
	requiredCalls := make(map[string]bool, len(requiredCallNames))
	for _, name := range requiredCallNames {
		requiredCalls[name] = false
	}
	forbiddenTypes := map[string]bool{
		"integrationDoctorGoReleaserBuild":          true,
		"integrationDoctorGoReleaserArchive":        true,
		"integrationDoctorGoReleaserFormatOverride": true,
		"integrationDoctorGoReleaserChecksum":       true,
		"integrationDoctorGoReleaserRelease":        true,
		"integrationDoctorGoReleaserConfig":         true,
	}
	foundCanonicalImport := false
	for _, parsed := range parseReleaseProductionFiles(t) {
		imports := make(map[string]string, len(parsed.file.Imports))
		for _, specification := range parsed.file.Imports {
			importPath, err := strconv.Unquote(specification.Path.Value)
			if err != nil {
				t.Fatalf("unquote import in %s: %v", parsed.path, err)
			}
			localName := path.Base(importPath)
			if specification.Name != nil {
				localName = specification.Name.Name
			}
			imports[localName] = importPath
			if importPath == canonicalImport {
				foundCanonicalImport = true
			}
		}
		ast.Inspect(parsed.file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.TypeSpec:
				if forbiddenTypes[typed.Name.Name] {
					t.Errorf("%s retains reusable GoReleaser DTO %s", parsed.path, typed.Name.Name)
				}
				tags := releaseArchitectureYAMLTags(typed.Type)
				for _, signature := range [][]string{
					{"project_name", "builds", "archives"},
					{"id", "binary", "main", "goos"},
					{"id", "ids", "formats", "name_template", "format_overrides"},
				} {
					if releaseArchitectureContainsAll(tags, signature...) {
						t.Errorf("%s declares alternate focused GoReleaser DTO %s", parsed.path, typed.Name.Name)
					}
				}
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				identifier, ok := selector.X.(*ast.Ident)
				if ok && imports[identifier.Name] == canonicalImport {
					if _, required := requiredCalls[selector.Sel.Name]; required {
						requiredCalls[selector.Sel.Name] = true
					}
				}
			}
			return true
		})
	}
	if !foundCanonicalImport {
		t.Fatal("Release production does not import canonical GoReleaser facts")
	}
	for _, call := range requiredCallNames {
		if !requiredCalls[call] {
			t.Errorf("Release production does not consume canonical goreleaser.%s", call)
		}
	}
}

func releaseArchitectureYAMLTags(expression ast.Expr) map[string]bool {
	structure, ok := expression.(*ast.StructType)
	if !ok {
		return nil
	}
	tags := make(map[string]bool)
	for _, field := range structure.Fields.List {
		if field.Tag == nil {
			continue
		}
		value, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			continue
		}
		name := strings.Split(reflect.StructTag(value).Get("yaml"), ",")[0]
		if name != "" && name != "-" {
			tags[name] = true
		}
	}
	return tags
}

func releaseArchitectureContainsAll(values map[string]bool, wants ...string) bool {
	for _, want := range wants {
		if !values[want] {
			return false
		}
	}
	return true
}

func TestIntegrationDoctorAvoidsGenericDiagnosticArchitecture(t *testing.T) {
	for _, path := range []string{
		"integration_doctor_consumer.go",
		"integration_doctor_credentials.go",
		"integration_doctor_goreleaser.go",
		"integration_doctor_inspection.go",
		"integration_doctor_installation.go",
		"integration_doctor_limitations.go",
		"integration_doctor_publication.go",
		"integration_doctor_repository_files.go",
		"integration_doctor_repository_identity.go",
		"integration_doctor_source.go",
		"integration_doctor_permissions.go",
		"integration_doctor_verification.go",
		"integration_doctor_workflow_inspection.go",
		"integration_doctor_response.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"DoctorManager",
			"DiagnosticManager",
			"InspectionManager",
			"WorkflowPipeline",
			"TransitionEngine",
			"StateMachine",
			"DependencyBag",
			"ServiceLocator",
			"CheckRegistry",
			"UnitOverview",
			"PipelineInspection",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited generic doctor architecture %q", path, forbidden)
			}
		}
	}
}

func TestIntegrationDoctorRemoteClientIsGETOnlyAndCannotReachMutationCapabilities(t *testing.T) {
	client := readCommandBoundarySource(t, "integration_doctor_github_read_client.go")
	if !strings.Contains(client, "http.MethodGet") || strings.Count(client, "httpClient.Do(request)") != 1 {
		t.Fatal("Doctor GitHub read client does not expose one bounded GET transport boundary")
	}
	for _, forbidden := range []string{
		"http.MethodPost",
		"http.MethodPut",
		"http.MethodPatch",
		"http.MethodDelete",
		"workflow_dispatch",
		"/dispatches",
		"/releases/latest",
		"CreateRelease",
		"UploadRelease",
		"UpdateVariable",
		"UpdateSecret",
		"UpdateWorkflow",
		"exec.Command",
		"os/exec",
		"os.WriteFile",
		"os.Mkdir",
		"os.Chdir",
	} {
		if strings.Contains(client, forbidden) {
			t.Fatalf("Doctor GitHub read client contains mutation capability %q", forbidden)
		}
	}
	for _, path := range []string{
		"integration_doctor_remote_inspection.go",
		"integration_doctor_remote_artifacts.go",
		"integration_doctor_remote_mapping.go",
		"integration_doctor_remote_result.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"GitHubActionsDispatcher",
			"GitHubActionsDispatchClient",
			"DispatchJournalStore",
			"ReleaseExecutionJournalStore",
			"EvidenceWriter",
			"os.WriteFile",
			"os.Mkdir",
			"os.Chdir",
			"os/exec",
			"exec.Command",
			"plugin.Response",
			"PipelineInspection",
			"DoctorRepair",
			"ProviderRegistry",
			"StateMachine",
			"DependencyBag",
			"ServiceLocator",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited remote Doctor capability %q", path, forbidden)
			}
		}
	}
}

func TestIntegrationDoctorDefaultPathGuardsRemoteAndTokenAccess(t *testing.T) {
	inspection := readCommandBoundarySource(t, "integration_doctor_inspection.go")
	if !strings.Contains(inspection, "if request.VerifyRemote && useCase.remote != nil") {
		t.Fatal("Doctor inspection does not guard its remote capability with the explicit request")
	}
	command := readCommandBoundarySource(t, "integration_doctor_command.go")
	if !strings.Contains(command, `request.Flags["verify-remote"]`) ||
		!strings.Contains(command, "EnvironmentGitHubActionsDispatchTokenResolver") {
		t.Fatal("Doctor command does not own explicit remote request and token composition")
	}
	for _, path := range []string{
		"integration_doctor_inspection.go",
		"integration_doctor_consumer.go",
		"integration_doctor_installation.go",
		"integration_doctor_credentials.go",
		"integration_doctor_publication.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{"os.Getenv", "os.LookupEnv", "Authorization", "Bearer "} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("default/local Doctor path %s contains token access %q", path, forbidden)
			}
		}
	}
}

func TestIntegrationDoctorRemoteUsesExactIdentitiesWithoutDiscoveryFallbacks(t *testing.T) {
	combined := readCommandBoundarySource(t, "integration_doctor_remote_inspection.go") +
		readCommandBoundarySource(t, "integration_doctor_remote_artifacts.go") +
		readCommandBoundarySource(t, "integration_doctor_github_read_client.go")
	for _, forbidden := range []string{
		"/releases/latest",
		"per_page",
		"newest",
		"latest matching",
		"git tag --list",
		"FindUnresolved",
		"ReleaseExecutionJournal",
		"DispatchJournal",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("remote Doctor contains discovery or journal fallback %q", forbidden)
		}
	}
	if strings.Contains(readCommandBoundarySource(t, "integration_doctor_remote_inspection.go"), ".WorkflowRun(") {
		t.Fatal("remote Doctor queries a workflow run without a durable Doctor-owned run id")
	}
}

func TestIntegrationDoctorPresentationKeepsCoreDomainNeutral(t *testing.T) {
	for _, path := range []string{
		"../../../../internal/terminal/style.go",
		"../../../../pkg/plugin/types.go",
		"../../../../pkg/renderer/renderer.go",
		"../../../../pkg/renderer/responsive_table.go",
		"../../../../pkg/renderer/property_values.go",
		"../../../../pkg/renderer/style.go",
	} {
		source := strings.ToLower(readCommandBoundarySource(t, path))
		for _, forbidden := range []string{
			"doctor",
			"diagnostic",
			"documentmodel",
			"layoutdsl",
			"rendererregistry",
			"statemachine",
			"themeengine",
			"documenttree",
			"providerregistry",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited domain or framework term %q", path, forbidden)
			}
		}
	}
}

func TestUnitOverviewApplicationHasNoMutationWorkflowParserDoctorGitNetworkTokenOrStoreDependencies(t *testing.T) {
	for _, path := range []string{
		"local_v2_source.go",
		"unit_overview_source.go",
		"unit_overview_inspection.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"GITHUB_TOKEN",
			"TokenResolver",
			"GitHubActionsDispatcher",
			"GitHubActionsDispatchClient",
			"net/http",
			"os/exec",
			"exec.Command",
			"NewGitReleaseCoordinator",
			"os.WriteFile",
			"os.Mkdir",
			"os.Chdir",
			"os.Setenv",
			"os.Getenv",
			"os.LookupEnv",
			"ReleaseExecutionJournalStore",
			"DispatchJournalStore",
			"EvidenceWriter",
			"EvidenceStore",
			"yaml.Unmarshal",
			"gopkg.in/yaml",
			"integrationDoctorWorkflow",
			"inspectIntegrationDoctorWorkflow",
			"ReleaseExecutor",
			"plugin.Response",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited unit-overview dependency %q", path, forbidden)
			}
		}
	}
}

func TestUnitOverviewKeepsTypedCommandAndResponseBoundaries(t *testing.T) {
	command := readCommandBoundarySource(t, "unit_overview_command.go")
	for _, required := range []string{"parseUnitOverviewRequest", "handler.inspector.Inspect", "mapUnitOverviewResult"} {
		if !strings.Contains(command, required) {
			t.Fatalf("unit overview command boundary omits %q", required)
		}
	}
	for _, forbidden := range []string{"LoadV2Config", "yaml.Unmarshal", "deriveCanonicalUnitOverviewRows", "derivePartialUnitOverviewRows"} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("unit overview command boundary contains application responsibility %q", forbidden)
		}
	}
	response := readCommandBoundarySource(t, "unit_overview_response.go")
	if !strings.Contains(response, "plugin.Response") {
		t.Fatal("unit overview response mapper does not own the plugin response boundary")
	}
}

func TestUnitOverviewDoesNotReachDoctorWorkflowInspection(t *testing.T) {
	for _, path := range []string{
		"unit_overview_command.go",
		"unit_overview_source.go",
		"unit_overview_inspection.go",
		"unit_overview_response.go",
		"unit_overview_types.go",
	} {
		if source := readCommandBoundarySource(t, path); strings.Contains(source, "integrationDoctor") {
			t.Fatalf("%s depends on integration-doctor orchestration", path)
		}
	}
}

func TestUnitOverviewAvoidsGenericInventoryArchitecture(t *testing.T) {
	for _, path := range []string{
		"local_v2_source.go",
		"unit_overview_command.go",
		"unit_overview_source.go",
		"unit_overview_inspection.go",
		"unit_overview_response.go",
		"unit_overview_types.go",
	} {
		source := readCommandBoundarySource(t, path)
		for _, forbidden := range []string{
			"StateMachine",
			"stateMachine",
			"DiagnosticRegistry",
			"InventoryRegistry",
			"ProviderRegistry",
			"InventoryFramework",
			"TableDSL",
			"DependencyBag",
			"ServiceLocator",
			"VisitorFramework",
			"PipelineInspection",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains prohibited unit-overview architecture %q", path, forbidden)
			}
		}
	}
}

func TestReleaseProgressReporterIsInfallible(t *testing.T) {
	source := readCommandBoundarySource(t, "release_progress.go")
	if !strings.Contains(source, "ReportReleaseProgress(event ReleaseProgressEvent)") {
		t.Fatal("ReleaseProgress must remain a narrow typed event reporter")
	}
	if strings.Contains(source, "ReportReleaseProgress(event ReleaseProgressEvent) error") {
		t.Fatal("ReleaseProgress reporter must not return errors")
	}
}

func readCommandBoundarySource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
