package doctor

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestIntegrationDoctorDefaultModeDoesNotInvokeRemoteInspectorOrTokenResolver(t *testing.T) {
	root := repositoryInspectionRoot(t)
	remoteCalls := 0
	tokenCalls := 0
	useCase := productionIntegrationDoctorUseCaseForTest(t, integrationDoctorRecordingRemoteInspector{
		calls: &remoteCalls,
		tokens: integrationDoctorRecordingTokenResolver{
			calls: &tokenCalls,
		},
	})
	result := useCase.Inspect(context.Background(), integrationDoctorRequest{RepositoryRoot: root.Path()})
	if remoteCalls != 0 || tokenCalls != 0 {
		t.Fatalf("remote calls=%d token calls=%d", remoteCalls, tokenCalls)
	}
	if result.RemoteVerification != (integrationDoctorRemoteSummary{Status: integrationDoctorRemoteNotRequested}) {
		t.Fatalf("remote summary=%#v", result.RemoteVerification)
	}
	if result.Readiness != integrationDoctorReady || result.Summary != (integrationDoctorSummary{NotVerifiable: 21, Verified: 15}) {
		t.Fatalf("readiness=%q summary=%#v", result.Readiness, result.Summary)
	}
}

func TestIntegrationDoctorExplicitRemoteVerificationResolvesSupportedFactsWithGETOnlyRequests(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, requests := newSuccessfulIntegrationDoctorGitHubServer(t, root.Path(), false)
	defer server.Close()
	tokenCalls := 0
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{calls: &tokenCalls, value: "remote-doctor-token-sentinel"},
		"",
	)
	if result.RemoteVerification.Status != integrationDoctorRemoteComplete || result.RemoteVerification.Verified == 0 ||
		result.RemoteVerification.Unresolved != 0 || result.RemoteVerification.Failed != 0 {
		t.Fatalf("remote summary=%#v", result.RemoteVerification)
	}
	if result.Readiness != integrationDoctorReady || result.Summary.Errors != 0 || result.Summary.Warnings != 0 {
		t.Fatalf("readiness=%q summary=%#v", result.Readiness, result.Summary)
	}
	if tokenCalls != 1 {
		t.Fatalf("token resolutions=%d, want 1", tokenCalls)
	}
	for _, code := range []string{"REMOTE_WORKFLOW_NOT_VERIFIABLE", "REPOSITORY_VARIABLES_NOT_VERIFIABLE"} {
		if integrationDoctorHasCode(result.Diagnostics, code) {
			t.Fatalf("successful remote result retained offline diagnostic %s", code)
		}
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "INSTALLATION_ARTIFACTS_NOT_VERIFIABLE" && !strings.Contains(diagnostic.Message, "remotely verified") {
			t.Fatalf("installation limitation was not narrowed: %#v", diagnostic)
		}
		if diagnostic.Code == "PUBLICATION_TARGET_NOT_VERIFIABLE" && !strings.Contains(diagnostic.Message, "remotely observed") {
			t.Fatalf("publication limitation was not narrowed: %#v", diagnostic)
		}
	}
	seenVariables := map[string]int{}
	for _, request := range requests.snapshot() {
		if request.method != http.MethodGet {
			t.Fatalf("method=%q uri=%s", request.method, request.uri)
		}
		if strings.Contains(request.uri, "/actions/secrets/GITHUB_TOKEN") ||
			strings.Contains(request.uri, "/releases/latest") || strings.Contains(request.uri, "/actions/runs") {
			t.Fatalf("forbidden request=%s", request.uri)
		}
		if strings.Contains(request.uri, "/actions/variables/") {
			seenVariables[request.uri]++
		}
	}
	if !reflect.DeepEqual(seenVariables, map[string]int{
		"/repos/nekoman-hq/neko-cli/actions/variables/NEKO_RELEASE_PLUGIN_VERSION": 1,
		"/repos/nekoman-hq/neko-cli/actions/variables/NEKO_VERSION":                1,
	}) {
		t.Fatalf("variable requests=%#v", seenVariables)
	}

	response := mapIntegrationDoctorResult(result, fixedReleaseClock{}.Now())
	var output bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &output); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"remote-doctor-token-sentinel", "Authorization", "Bearer", root.Path(), "\x1b["} {
		if strings.Contains(output.String(), forbidden) {
			t.Fatalf("JSON contains %q", forbidden)
		}
	}
	for _, required := range []string{`"remote_verification"`, `"requested": true`, `"status": "complete"`} {
		if !strings.Contains(output.String(), required) {
			t.Fatalf("JSON omits %q: %s", required, output.String())
		}
	}
	rawResponse := *response
	rawResponse.RendererHint = "raw-json"
	var rawOutput bytes.Buffer
	if err := renderer.RenderWithOptionsTo(&rawResponse, renderer.RenderOptions{Format: renderer.FormatTable}, &rawOutput); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(rawOutput.Bytes()) {
		t.Fatalf("raw output is not JSON: %s", rawOutput.String())
	}
	for _, forbidden := range []string{"remote-doctor-token-sentinel", "Authorization", "Bearer", "\x1b["} {
		if strings.Contains(rawOutput.String(), forbidden) {
			t.Fatalf("raw JSON contains %q", forbidden)
		}
	}
}

func TestIntegrationDoctorPrivateRepositoryRetriesIdentityOnceWithAuthentication(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, requests := newSuccessfulIntegrationDoctorGitHubServer(t, root.Path(), true)
	defer server.Close()
	tokenCalls := 0
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{calls: &tokenCalls, value: "private-read-token"},
		"cli",
	)
	if result.RemoteVerification.Status != integrationDoctorRemoteComplete || result.Readiness != integrationDoctorReady {
		t.Fatalf("remote=%#v readiness=%q diagnostics=%#v", result.RemoteVerification, result.Readiness, result.Diagnostics)
	}
	if tokenCalls != 1 {
		t.Fatalf("token calls=%d", tokenCalls)
	}
	repositoryRequests := make([]integrationDoctorRecordedRequest, 0)
	for _, request := range requests.snapshot() {
		if request.uri == "/repos/nekoman-hq/neko-cli" {
			repositoryRequests = append(repositoryRequests, request)
		}
	}
	if len(repositoryRequests) != 2 || repositoryRequests[0].authorization != "" ||
		repositoryRequests[1].authorization != "Bearer private-read-token" {
		t.Fatalf("repository requests=%#v", repositoryRequests)
	}
}

func TestIntegrationDoctorExplicitRemoteVerificationSupportsEveryUnitScope(t *testing.T) {
	for _, unitID := range []string{"cli", "plugin-release", "plugin-ui"} {
		t.Run(unitID, func(t *testing.T) {
			root := repositoryInspectionRoot(t)
			server, _ := newSuccessfulIntegrationDoctorGitHubServer(t, root.Path(), false)
			defer server.Close()
			result := runIntegrationDoctorRemoteAgainstServer(
				t, root.Path(), server.URL,
				integrationDoctorRecordingTokenResolver{value: "unit-scope-token"},
				unitID,
			)
			if result.RemoteVerification.Status != integrationDoctorRemoteComplete || result.Readiness != integrationDoctorReady ||
				len(result.Units) != 1 || result.Units[0].ID != unitID || len(result.Workflows) != 1 {
				t.Fatalf("remote=%#v readiness=%q units=%#v workflows=%#v", result.RemoteVerification, result.Readiness, result.Units, result.Workflows)
			}
		})
	}
}

func TestIntegrationDoctorRemoteAccessFailuresRemainPartialAndPreserveVerifiedFacts(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, func(writer http.ResponseWriter, request *http.Request) bool {
		if strings.HasSuffix(request.URL.Path, "/actions/permissions") {
			writer.WriteHeader(http.StatusForbidden)
			_, _ = fmt.Fprint(writer, `{"message":"protected policy body"}`)
			return true
		}
		if strings.HasSuffix(request.URL.Path, "/actions/variables/NEKO_RELEASE_PLUGIN_VERSION") {
			writer.Header().Set("Retry-After", "15")
			writer.WriteHeader(http.StatusTooManyRequests)
			_, _ = fmt.Fprint(writer, `{"message":"rate limited private body"}`)
			return true
		}
		return false
	})
	defer server.Close()
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{value: "partial-token"},
		"cli",
	)
	if result.RemoteVerification.Status != integrationDoctorRemotePartial || result.RemoteVerification.Verified == 0 ||
		result.RemoteVerification.Unresolved == 0 || result.Readiness != integrationDoctorReady {
		t.Fatalf("remote=%#v readiness=%q summary=%#v", result.RemoteVerification, result.Readiness, result.Summary)
	}
	for _, code := range []string{"REMOTE_ACTIONS_POLICY_UNAUTHORIZED", "REMOTE_REPOSITORY_VARIABLE_RATE_LIMITED"} {
		if !integrationDoctorHasCode(result.Diagnostics, code) {
			t.Fatalf("missing %s in %#v", code, result.Diagnostics)
		}
	}
	verifiedWorkflow := false
	for _, fact := range result.Verifications {
		if fact.Category == "remote_workflow_identity" && strings.HasSuffix(fact.Subject, "#content") && fact.State == integrationDoctorVerified {
			verifiedWorkflow = true
		}
	}
	if !verifiedWorkflow {
		t.Fatal("partial failure erased verified workflow content fact")
	}
}

func TestIntegrationDoctorDefiniteRemoteWorkflowMismatchIsActionable(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, func(writer http.ResponseWriter, request *http.Request) bool {
		if strings.Contains(request.URL.Path, "/contents/.github/workflows/release-neko-cli.yml") {
			_, _ = fmt.Fprintf(writer, `{"type":"file","path":".github/workflows/release-neko-cli.yml","encoding":"base64","content":%q}`,
				base64.StdEncoding.EncodeToString([]byte("name: changed-remotely\n")))
			return true
		}
		return false
	})
	defer server.Close()
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{value: "mismatch-token"},
		"cli",
	)
	if result.Readiness != integrationDoctorNotReady || result.Summary.Errors == 0 ||
		!integrationDoctorHasCode(result.Diagnostics, "REMOTE_WORKFLOW_CONTENT_MISMATCH") {
		t.Fatalf("readiness=%q summary=%#v diagnostics=%#v", result.Readiness, result.Summary, result.Diagnostics)
	}
	if response := mapIntegrationDoctorResult(result, fixedReleaseClock{}.Now()); response.ExitCode != 1 {
		t.Fatalf("remote mismatch exit=%d", response.ExitCode)
	}
}

func TestIntegrationDoctorUnavailablePublicationTargetDoesNotEraseOtherRemoteEvidence(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, func(writer http.ResponseWriter, request *http.Request) bool {
		if strings.HasSuffix(request.URL.Path, "/releases/tags/plugin-ui/v1.0.1") {
			writer.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprint(writer, `{"message":"private publication service body"}`)
			return true
		}
		return false
	})
	defer server.Close()
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{value: "publication-read-token"},
		"plugin-ui",
	)
	if result.RemoteVerification.Status != integrationDoctorRemotePartial || result.Readiness != integrationDoctorReady ||
		!integrationDoctorHasCode(result.Diagnostics, "REMOTE_PUBLICATION_RELEASE_UNAVAILABLE") {
		t.Fatalf("remote=%#v readiness=%q diagnostics=%#v", result.RemoteVerification, result.Readiness, result.Diagnostics)
	}
	verifiedWorkflow := false
	for _, fact := range result.Verifications {
		if fact.Category == "remote_workflow_identity" && fact.State == integrationDoctorVerified {
			verifiedWorkflow = true
		}
	}
	if !verifiedWorkflow {
		t.Fatal("publication target outage erased successful repository/workflow evidence")
	}
}

func TestIntegrationDoctorAmbiguousAnonymousRepositoryNotFoundIsNotMissing(t *testing.T) {
	root := repositoryInspectionRoot(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(writer, `{"message":"not found"}`)
	}))
	defer server.Close()
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{err: errors.New("no token")},
		"cli",
	)
	if result.Readiness != integrationDoctorReady || requests != 1 ||
		!integrationDoctorHasCode(result.Diagnostics, "REMOTE_REPOSITORY_UNAUTHORIZED") ||
		integrationDoctorHasCode(result.Diagnostics, "REMOTE_REPOSITORY_MISSING") {
		t.Fatalf("readiness=%q requests=%d diagnostics=%#v", result.Readiness, requests, result.Diagnostics)
	}
}

func TestIntegrationDoctorDefiniteAuthenticatedRepositoryNotFoundIsMissing(t *testing.T) {
	root := repositoryInspectionRoot(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{value: "authenticated-read-token"},
		"cli",
	)
	if result.Readiness != integrationDoctorNotReady || requests != 2 ||
		!integrationDoctorHasCode(result.Diagnostics, "REMOTE_REPOSITORY_MISSING") {
		t.Fatalf("readiness=%q requests=%d diagnostics=%#v", result.Readiness, requests, result.Diagnostics)
	}
}

func TestIntegrationDoctorRemoteRepositoryIdentityMismatchAndUnavailableAreDistinct(t *testing.T) {
	//nolint:govet // Table fields follow remote response input then expected result.
	tests := []struct {
		name      string
		status    int
		payload   string
		wantCode  string
		readiness integrationDoctorReadiness
		remote    integrationDoctorRemoteStatus
	}{
		{
			name: "identity mismatch", status: http.StatusOK,
			payload:  `{"name":"other","owner":{"login":"acme"},"default_branch":"main","visibility":"public","private":false}`,
			wantCode: "REMOTE_REPOSITORY_IDENTITY_MISMATCH", readiness: integrationDoctorNotReady,
			remote: integrationDoctorRemotePartial,
		},
		{
			name: "unavailable", status: http.StatusServiceUnavailable,
			payload:  `{"message":"private service body"}`,
			wantCode: "REMOTE_REPOSITORY_UNAVAILABLE", readiness: integrationDoctorReady,
			remote: integrationDoctorRemoteUnavailable,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := repositoryInspectionRoot(t)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = fmt.Fprint(writer, test.payload)
			}))
			defer server.Close()
			result := runIntegrationDoctorRemoteAgainstServer(
				t, root.Path(), server.URL,
				integrationDoctorRecordingTokenResolver{value: "repository-read-token"}, "cli",
			)
			if result.Readiness != test.readiness || result.RemoteVerification.Status != test.remote ||
				!integrationDoctorHasCode(result.Diagnostics, test.wantCode) {
				t.Fatalf("readiness=%q remote=%#v diagnostics=%#v", result.Readiness, result.RemoteVerification, result.Diagnostics)
			}
		})
	}
}

func TestIntegrationDoctorQueriesOnlyRecognizedVersionVariables(t *testing.T) {
	workflow := integrationDoctorRemoteWorkflowFromYAML(t, ".github/workflows/release.yml", `
jobs:
  verify:
    steps:
      - name: install cli
        env:
          NEKO_VERSION: ${{ vars.NEKO_VERSION }}
        run: |
          echo "${{ vars.UNRELATED }}"
          curl install.sh
          echo "$NEKO_VERSION"
      - name: install plugin
        env:
          NEKO_RELEASE_PLUGIN_VERSION: ${{ vars.NEKO_RELEASE_PLUGIN_VERSION }}
        run: neko plugin install release --version "$NEKO_RELEASE_PLUGIN_VERSION"
`)
	//nolint:govet // Table fields follow remote input then expected classification.
	tests := []struct {
		name            string
		variable        string
		status          int
		value           string
		wantState       integrationDoctorVerificationState
		wantCode        string
		rateLimitHeader bool
	}{
		{name: "valid", variable: "NEKO_VERSION", status: http.StatusOK, value: "v3.0.4", wantState: integrationDoctorVerified},
		{name: "missing", variable: "NEKO_VERSION", status: http.StatusNotFound, wantState: integrationDoctorMissing, wantCode: "REMOTE_REPOSITORY_VARIABLE_MISSING"},
		{name: "invalid", variable: "NEKO_VERSION", status: http.StatusOK, value: "latest", wantState: integrationDoctorMismatch, wantCode: "REMOTE_REPOSITORY_VARIABLE_INVALID"},
		{name: "unauthorized", variable: "NEKO_VERSION", status: http.StatusForbidden, wantState: integrationDoctorUnauthorized, wantCode: "REMOTE_REPOSITORY_VARIABLE_UNAUTHORIZED"},
		{name: "rate limited", variable: "NEKO_VERSION", status: http.StatusTooManyRequests, wantState: integrationDoctorRateLimited, wantCode: "REMOTE_REPOSITORY_VARIABLE_RATE_LIMITED", rateLimitHeader: true},
		{name: "unavailable", variable: "NEKO_VERSION", status: http.StatusServiceUnavailable, wantState: integrationDoctorUnavailable, wantCode: "REMOTE_REPOSITORY_VARIABLE_UNAVAILABLE"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				paths = append(paths, request.URL.Path)
				name := strings.TrimPrefix(request.URL.Path, "/repos/acme/example/actions/variables/")
				if name != test.variable {
					_, _ = fmt.Fprintf(writer, `{"name":%q,"value":"4.2.0"}`, name)
					return
				}
				if test.rateLimitHeader {
					writer.Header().Set("Retry-After", "10")
				}
				writer.WriteHeader(test.status)
				if test.status == http.StatusOK {
					_, _ = fmt.Fprintf(writer, `{"name":%q,"value":%q}`, name, test.value)
				}
			}))
			defer server.Close()
			client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
			inspection := integrationDoctorRemoteInspection{}
			inspector := integrationDoctorGitHubRemoteInspector{reader: client}
			remote := &integrationDoctorRemoteContext{
				identity:       integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"},
				protected:      &integrationDoctorRemoteTokenAccess{resolver: integrationDoctorRecordingTokenResolver{value: "variables-token"}},
				variableValues: make(map[string]string),
			}
			inspector.inspectVariables(context.Background(), remote, []integrationDoctorRemoteWorkflow{workflow}, &inspection)
			var fact *integrationDoctorVerification
			for index := range inspection.Verifications {
				if inspection.Verifications[index].Subject == test.variable {
					fact = &inspection.Verifications[index]
				}
			}
			if fact == nil || fact.State != test.wantState {
				t.Fatalf("fact=%#v verifications=%#v", fact, inspection.Verifications)
			}
			if test.wantCode != "" && !integrationDoctorHasCode(inspection.Diagnostics, test.wantCode) {
				t.Fatalf("diagnostics=%#v", inspection.Diagnostics)
			}
			for _, path := range paths {
				if strings.Contains(path, "UNRELATED") {
					t.Fatalf("unrelated variable queried: %s", path)
				}
			}
		})
	}
}

func TestIntegrationDoctorQueriesOnlyReferencedCustomSecretNames(t *testing.T) {
	workflow := integrationDoctorRemoteWorkflowFromYAML(t, ".github/workflows/release.yml", `
jobs:
  publish:
    steps:
      - name: publish
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          SIGNING_TOKEN: ${{ secrets.SIGNING_TOKEN }}
        run: gh release create "$RELEASE_TAG"
`)
	//nolint:govet // Table fields follow remote input then expected classification.
	tests := []struct {
		name      string
		status    int
		wantState integrationDoctorVerificationState
		wantCode  string
	}{
		{name: "present", status: http.StatusOK, wantState: integrationDoctorVerified},
		{name: "missing", status: http.StatusNotFound, wantState: integrationDoctorMissing, wantCode: "REMOTE_SECRET_METADATA_MISSING"},
		{name: "unauthorized", status: http.StatusForbidden, wantState: integrationDoctorUnauthorized, wantCode: "REMOTE_SECRET_METADATA_UNAUTHORIZED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				paths = append(paths, request.URL.Path)
				writer.WriteHeader(test.status)
				if test.status == http.StatusOK {
					_, _ = fmt.Fprint(writer, `{"name":"SIGNING_TOKEN","created_at":"2026-07-01T00:00:00Z","updated_at":"2026-07-02T00:00:00Z","value":"secret-value-must-not-exist"}`)
				}
			}))
			defer server.Close()
			client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
			inspection := integrationDoctorRemoteInspection{}
			inspector := integrationDoctorGitHubRemoteInspector{reader: client}
			remote := &integrationDoctorRemoteContext{
				identity:  integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"},
				protected: &integrationDoctorRemoteTokenAccess{resolver: integrationDoctorRecordingTokenResolver{value: "secret-metadata-token"}},
			}
			inspector.inspectSecrets(context.Background(), remote, []integrationDoctorRemoteWorkflow{workflow}, &inspection)
			if len(inspection.Verifications) != 1 || inspection.Verifications[0].Subject != "SIGNING_TOKEN" ||
				inspection.Verifications[0].State != test.wantState {
				t.Fatalf("verifications=%#v", inspection.Verifications)
			}
			if test.wantCode != "" && !integrationDoctorHasCode(inspection.Diagnostics, test.wantCode) {
				t.Fatalf("diagnostics=%#v", inspection.Diagnostics)
			}
			serialized := fmt.Sprintf("%#v %#v", inspection.Verifications, inspection.Diagnostics)
			for _, forbidden := range []string{"secret-value-must-not-exist", "secret-metadata-token"} {
				if strings.Contains(serialized, forbidden) {
					t.Fatalf("remote secret result contains %q", forbidden)
				}
			}
			if len(paths) != 1 || strings.Contains(paths[0], "GITHUB_TOKEN") || !strings.HasSuffix(paths[0], "/SIGNING_TOKEN") {
				t.Fatalf("secret paths=%#v", paths)
			}
		})
	}
}

func TestIntegrationDoctorActionsPolicyDisabledIsIndependentRemoteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(writer, `{"enabled":false,"allowed_actions":"all"}`)
	}))
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	inspection := integrationDoctorRemoteInspection{}
	inspector := integrationDoctorGitHubRemoteInspector{reader: client}
	remote := &integrationDoctorRemoteContext{
		identity:  integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"},
		protected: &integrationDoctorRemoteTokenAccess{resolver: integrationDoctorRecordingTokenResolver{value: "policy-token"}},
	}
	inspector.inspectActionsPolicy(context.Background(), remote, &inspection)
	if len(inspection.Verifications) != 1 || inspection.Verifications[0].State != integrationDoctorMismatch ||
		!integrationDoctorHasCode(inspection.Diagnostics, "REMOTE_ACTIONS_DISABLED") {
		t.Fatalf("inspection=%#v", inspection)
	}
}

func TestIntegrationDoctorActionsPolicyUnsupportedRemainsUnresolved(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(writer, `{"enabled":true,"allowed_actions":"future_policy"}`)
	}))
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	inspection := integrationDoctorRemoteInspection{}
	inspector := integrationDoctorGitHubRemoteInspector{reader: client}
	remote := &integrationDoctorRemoteContext{
		identity:  integrationDoctorRepositoryIdentity{Owner: "acme", Repository: "example"},
		protected: &integrationDoctorRemoteTokenAccess{resolver: integrationDoctorRecordingTokenResolver{value: "policy-token"}},
	}
	inspector.inspectActionsPolicy(context.Background(), remote, &inspection)
	if len(inspection.Verifications) != 1 || inspection.Verifications[0].State != integrationDoctorUnsupported ||
		!integrationDoctorHasCode(inspection.Diagnostics, "REMOTE_ACTIONS_POLICY_UNSUPPORTED") ||
		inspection.Diagnostics[0].Severity != integrationDoctorNotVerifiable {
		t.Fatalf("inspection=%#v", inspection)
	}
}

func TestIntegrationDoctorMissingInstallationAssetReplacesAvailabilityLimitationWithError(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, func(writer http.ResponseWriter, request *http.Request) bool {
		if strings.HasSuffix(request.URL.Path, "/releases/tags/v3.0.4") {
			writeIntegrationDoctorReleaseFixture(writer, "v3.0.4", []string{"neko-cli_Darwin_arm64.tar.gz"})
			return true
		}
		return false
	})
	defer server.Close()
	result := runIntegrationDoctorRemoteAgainstServer(
		t, root.Path(), server.URL,
		integrationDoctorRecordingTokenResolver{value: "asset-token"},
		"cli",
	)
	if result.Readiness != integrationDoctorNotReady ||
		!integrationDoctorHasCode(result.Diagnostics, "REMOTE_INSTALLATION_ASSET_MISSING") ||
		integrationDoctorHasCode(result.Diagnostics, "INSTALLATION_ARTIFACTS_NOT_VERIFIABLE") {
		t.Fatalf("readiness=%q diagnostics=%#v", result.Readiness, result.Diagnostics)
	}
}

func TestIntegrationDoctorReleasePluginInstallationRequiresExactArchiveAndChecksumAssets(t *testing.T) {
	//nolint:govet // Table fields follow the fixture label then its mutation predicate.
	for _, test := range []struct {
		name   string
		remove func(string) bool
	}{
		{name: "plugin archive missing", remove: func(asset string) bool { return strings.Contains(asset, "Darwin_arm64") }},
		{name: "plugin checksum missing", remove: func(asset string) bool { return strings.HasSuffix(asset, "_checksums.txt") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := repositoryInspectionRoot(t)
			fixtures := integrationDoctorRemoteReleaseFixtures(t, root.Path())
			assets := make([]string, 0, len(fixtures["plugin-release/v4.2.0"]))
			for _, asset := range fixtures["plugin-release/v4.2.0"] {
				if !test.remove(asset) {
					assets = append(assets, asset)
				}
			}
			server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.HasSuffix(request.URL.Path, "/releases/tags/plugin-release/v4.2.0") {
					writeIntegrationDoctorReleaseFixture(writer, "plugin-release/v4.2.0", assets)
					return true
				}
				return false
			})
			defer server.Close()
			result := runIntegrationDoctorRemoteAgainstServer(
				t, root.Path(), server.URL,
				integrationDoctorRecordingTokenResolver{value: "plugin-asset-token"}, "plugin-release",
			)
			if result.Readiness != integrationDoctorNotReady ||
				!integrationDoctorHasCode(result.Diagnostics, "REMOTE_INSTALLATION_ASSET_MISSING") {
				t.Fatalf("readiness=%q diagnostics=%#v", result.Readiness, result.Diagnostics)
			}
		})
	}
}

func TestIntegrationDoctorDefiniteRemoteReleaseAndTagFailuresAreActionable(t *testing.T) {
	tests := []struct {
		name     string
		pathEnds string
		wantCode string
	}{
		{name: "installation release missing", pathEnds: "/releases/tags/v3.0.4", wantCode: "REMOTE_INSTALLATION_RELEASE_MISSING"},
		{name: "publication release missing", pathEnds: "/releases/tags/v3.0.4", wantCode: "REMOTE_PUBLICATION_RELEASE_MISSING"},
		{name: "publication tag missing", pathEnds: "/git/ref/tags/v3.0.4", wantCode: "REMOTE_PUBLICATION_TAG_MISSING"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := repositoryInspectionRoot(t)
			server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.HasSuffix(request.URL.Path, test.pathEnds) {
					writer.WriteHeader(http.StatusNotFound)
					return true
				}
				return false
			})
			defer server.Close()
			result := runIntegrationDoctorRemoteAgainstServer(
				t, root.Path(), server.URL,
				integrationDoctorRecordingTokenResolver{value: "artifact-token"}, "cli",
			)
			if result.Readiness != integrationDoctorNotReady || !integrationDoctorHasCode(result.Diagnostics, test.wantCode) {
				t.Fatalf("readiness=%q diagnostics=%#v", result.Readiness, result.Diagnostics)
			}
		})
	}
}

func TestIntegrationDoctorRemoteWorkflowMissingDisabledAndUnsupportedStates(t *testing.T) {
	tests := []struct {
		name       string
		override   func(http.ResponseWriter, *http.Request) bool
		wantCode   string
		wantErrors bool
	}{
		{
			name: "missing",
			override: func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.Contains(request.URL.Path, "/contents/.github/workflows/release-neko-cli.yml") {
					writer.WriteHeader(http.StatusNotFound)
					return true
				}
				return false
			},
			wantCode: "REMOTE_WORKFLOW_CONTENT_MISSING", wantErrors: true,
		},
		{
			name: "disabled",
			override: func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.HasSuffix(request.URL.Path, "/actions/workflows/release-neko-cli.yml") {
					_, _ = fmt.Fprint(writer, `{"path":".github/workflows/release-neko-cli.yml","state":"disabled_manually"}`)
					return true
				}
				return false
			},
			wantCode: "REMOTE_WORKFLOW_DISABLED", wantErrors: true,
		},
		{
			name: "unsupported",
			override: func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.HasSuffix(request.URL.Path, "/actions/workflows/release-neko-cli.yml") {
					_, _ = fmt.Fprint(writer, `{"path":".github/workflows/release-neko-cli.yml","state":"future_state"}`)
					return true
				}
				return false
			},
			wantCode: "REMOTE_WORKFLOW_STATE_UNSUPPORTED",
		},
		{
			name: "unauthorized",
			override: func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.Contains(request.URL.Path, "/contents/.github/workflows/release-neko-cli.yml") {
					writer.WriteHeader(http.StatusForbidden)
					return true
				}
				return false
			},
			wantCode: "REMOTE_WORKFLOW_CONTENT_UNAUTHORIZED",
		},
		{
			name: "rate limited",
			override: func(writer http.ResponseWriter, request *http.Request) bool {
				if strings.Contains(request.URL.Path, "/contents/.github/workflows/release-neko-cli.yml") {
					writer.Header().Set("Retry-After", "20")
					writer.WriteHeader(http.StatusTooManyRequests)
					return true
				}
				return false
			},
			wantCode: "REMOTE_WORKFLOW_CONTENT_RATE_LIMITED",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := repositoryInspectionRoot(t)
			server, _ := newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, root.Path(), false, test.override)
			defer server.Close()
			result := runIntegrationDoctorRemoteAgainstServer(
				t, root.Path(), server.URL,
				integrationDoctorRecordingTokenResolver{value: "workflow-state-token"},
				"cli",
			)
			if !integrationDoctorHasCode(result.Diagnostics, test.wantCode) {
				t.Fatalf("diagnostics=%#v", result.Diagnostics)
			}
			if test.wantErrors && result.Readiness != integrationDoctorNotReady {
				t.Fatalf("readiness=%q", result.Readiness)
			}
			if !test.wantErrors && result.Readiness != integrationDoctorReady {
				t.Fatalf("readiness=%q", result.Readiness)
			}
		})
	}
}

func TestIntegrationDoctorRemoteHumanPresentationIsResponsiveAndSemantic(t *testing.T) {
	result := integrationDoctorResult{
		RemoteVerification: integrationDoctorRemoteSummary{
			Requested: true, Status: integrationDoctorRemotePartial, Verified: 8, Unresolved: 2, Failed: 1,
		},
		Verifications: []integrationDoctorVerification{
			integrationDoctorRemoteFact("workflow", "remote_workflow_identity", integrationDoctorVerified, "verified", ".github/workflows/release.yml", "", ".github/workflows/release.yml"),
		},
		Diagnostics: []integrationDoctorDiagnostic{
			*integrationDoctorRemoteDiagnostic(integrationDoctorNotVerifiable, "REMOTE_ACTIONS_POLICY_UNAUTHORIZED", ".github/workflows/release.yml", "", "unresolved remote policy", "supply protected read access"),
		},
	}
	finalizeIntegrationDoctorResult(&result)
	response := mapIntegrationDoctorResultForTest(result)
	for _, width := range []int{120, 60, 40} {
		output := renderIntegrationDoctorWithColorForTest(t, response, width, false)
		assertReleasePlanLinesFit(t, output, width)
		normalized := strings.Join(strings.Fields(output), " ")
		for _, required := range []string{"Remote", "verification", "partial", "verified", "unresolved", "failed"} {
			if !strings.Contains(normalized, required) {
				t.Fatalf("width %d omits %q:\n%s", width, required, output)
			}
		}
		if strings.Contains(output, "\x1b[") {
			t.Fatalf("plain output contains ANSI: %q", output)
		}
	}
	unknown := renderReleasePlanForTest(t, response, renderer.FormatTable, releasePlanOutputWidth{})
	if !strings.Contains(unknown, "Remote verification") || strings.Contains(unknown, "\x1b[") {
		t.Fatalf("unknown-width output=%q", unknown)
	}
	colored := renderIntegrationDoctorWithColorForTest(t, response, 100, true)
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("TTY output omitted semantic color: %q", colored)
	}
}

func integrationDoctorRemoteWorkflowFromYAML(
	t *testing.T,
	path string,
	content string,
) integrationDoctorRemoteWorkflow {
	t.Helper()
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(content), &document); err != nil {
		t.Fatal(err)
	}
	return integrationDoctorRemoteWorkflow{
		Path: path,
		Snapshot: integrationDoctorWorkflowSnapshot{
			Document: &document, Content: []byte(content), Exists: true,
		},
	}
}

type integrationDoctorRecordingRemoteInspector struct {
	calls  *int
	tokens integrationDoctorRecordingTokenResolver
}

func (inspector integrationDoctorRecordingRemoteInspector) Inspect(
	ctx context.Context,
	_ integrationDoctorRemoteRequest,
) integrationDoctorRemoteInspection {
	*inspector.calls++
	_, _ = inspector.tokens.ResolveGitHubReadToken(ctx)
	return integrationDoctorRemoteInspection{}
}

//nolint:govet // Test double fields follow observation, value, failure.
type integrationDoctorRecordingTokenResolver struct {
	calls *int
	value string
	err   error
}

func (resolver integrationDoctorRecordingTokenResolver) ResolveGitHubReadToken(
	_ context.Context,
) (githubReadToken, error) {
	if resolver.calls != nil {
		*resolver.calls++
	}
	if resolver.err != nil {
		return githubReadToken{}, resolver.err
	}
	return newGitHubReadToken(resolver.value)
}

type integrationDoctorRecordedRequest struct {
	method        string
	uri           string
	authorization string
}

type integrationDoctorRecordedRequests struct {
	values []integrationDoctorRecordedRequest
	lock   sync.Mutex
}

func (requests *integrationDoctorRecordedRequests) append(request *http.Request) {
	requests.lock.Lock()
	defer requests.lock.Unlock()
	requests.values = append(requests.values, integrationDoctorRecordedRequest{
		method: request.Method, uri: request.URL.RequestURI(), authorization: request.Header.Get("Authorization"),
	})
}

func (requests *integrationDoctorRecordedRequests) snapshot() []integrationDoctorRecordedRequest {
	requests.lock.Lock()
	defer requests.lock.Unlock()
	return append([]integrationDoctorRecordedRequest(nil), requests.values...)
}

func runIntegrationDoctorRemoteAgainstServer(
	t *testing.T,
	repositoryRoot string,
	baseURL string,
	resolver githubReadTokenResolver,
	unitID string,
) *integrationDoctorResult {
	t.Helper()
	client, err := newIntegrationDoctorGitHubReadClient(withIntegrationDoctorGitHubReadBaseURL(baseURL))
	if err != nil {
		t.Fatal(err)
	}
	useCase := productionIntegrationDoctorUseCaseForTest(t, integrationDoctorGitHubRemoteInspector{
		reader: client,
		tokens: resolver,
	})
	result := useCase.Inspect(context.Background(), integrationDoctorRequest{
		RepositoryRoot: repositoryRoot,
		UnitID:         unitID,
		VerifyRemote:   true,
	})
	return result
}

func productionIntegrationDoctorUseCaseForTest(
	t *testing.T,
	remote integrationDoctorRemoteInspector,
) integrationDoctorInspectionUseCase {
	t.Helper()
	return integrationDoctorInspectionUseCase{
		sources:    filesystemIntegrationDoctorSourceReader{},
		workflows:  filesystemIntegrationDoctorWorkflowReader{},
		files:      filesystemIntegrationDoctorRepositoryFileReader{},
		identities: filesystemIntegrationDoctorRepositoryIdentityReader{},
		remote:     remote,
	}
}

func newSuccessfulIntegrationDoctorGitHubServer(
	t *testing.T,
	repositoryRoot string,
	private bool,
) (*httptest.Server, *integrationDoctorRecordedRequests) {
	t.Helper()
	return newSuccessfulIntegrationDoctorGitHubServerWithOverrides(t, repositoryRoot, private, nil)
}

func newSuccessfulIntegrationDoctorGitHubServerWithOverrides(
	t *testing.T,
	repositoryRoot string,
	private bool,
	override func(http.ResponseWriter, *http.Request) bool,
) (*httptest.Server, *integrationDoctorRecordedRequests) {
	t.Helper()
	releases := integrationDoctorRemoteReleaseFixtures(t, repositoryRoot)
	requests := &integrationDoctorRecordedRequests{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.append(request)
		if private && request.URL.Path == "/repos/nekoman-hq/neko-cli" && request.Header.Get("Authorization") == "" {
			writer.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprint(writer, `{"message":"not found"}`)
			return
		}
		if private && request.Header.Get("Authorization") == "" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		if override != nil && override(writer, request) {
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/repos/nekoman-hq/neko-cli":
			_, _ = fmt.Fprintf(writer, `{"name":"neko-cli","owner":{"login":"nekoman-hq"},"default_branch":"main","visibility":%q,"private":%t}`,
				map[bool]string{true: "private", false: "public"}[private], private)
		case strings.HasPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/contents/"):
			workflow := strings.TrimPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/contents/")
			content, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(workflow)))
			if err != nil {
				http.NotFound(writer, request)
				return
			}
			_, _ = fmt.Fprintf(writer, `{"type":"file","path":%q,"encoding":"base64","content":%q}`,
				workflow, base64.StdEncoding.EncodeToString(content))
		case strings.HasPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/actions/workflows/"):
			filename := strings.TrimPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/actions/workflows/")
			_, _ = fmt.Fprintf(writer, `{"path":%q,"state":"active"}`, ".github/workflows/"+filename)
		case request.URL.Path == "/repos/nekoman-hq/neko-cli/actions/variables/NEKO_VERSION":
			_, _ = fmt.Fprint(writer, `{"name":"NEKO_VERSION","value":"3.0.4"}`)
		case request.URL.Path == "/repos/nekoman-hq/neko-cli/actions/variables/NEKO_RELEASE_PLUGIN_VERSION":
			_, _ = fmt.Fprint(writer, `{"name":"NEKO_RELEASE_PLUGIN_VERSION","value":"4.2.0"}`)
		case request.URL.Path == "/repos/nekoman-hq/neko-cli/actions/permissions":
			_, _ = fmt.Fprint(writer, `{"enabled":true,"allowed_actions":"all"}`)
		case strings.HasPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/releases/tags/"):
			tag := strings.TrimPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/releases/tags/")
			assets, exists := releases[tag]
			if !exists {
				http.NotFound(writer, request)
				return
			}
			writeIntegrationDoctorReleaseFixture(writer, tag, assets)
		case strings.HasPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/git/ref/tags/"):
			tag := strings.TrimPrefix(request.URL.Path, "/repos/nekoman-hq/neko-cli/git/ref/tags/")
			_, _ = fmt.Fprintf(writer, `{"ref":%q,"object":{"type":"commit","sha":"0123456789abcdef"}}`, "refs/tags/"+tag)
		default:
			http.NotFound(writer, request)
		}
	}))
	return server, requests
}

func integrationDoctorRemoteReleaseFixtures(
	t *testing.T,
	repositoryRoot string,
) map[string][]string {
	t.Helper()
	repository, err := releaseconfig.LoadReleaseRepository(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	files := filesystemIntegrationDoctorRepositoryFileReader{}
	request := integrationDoctorRemoteRequest{RepositoryRoot: repositoryRoot, Files: files}
	releases := map[string][]string{"plugin-registry": {"plugin-index.json"}}
	for _, unit := range repository.Units {
		contract, ok := integrationDoctorPublicationRemoteContract(request, unit)
		if !ok {
			t.Fatalf("publication contract missing for %s", unit.ID)
		}
		releases[contract.Tag] = append(releases[contract.Tag], contract.RequiredAssets...)
	}
	for _, contract := range integrationDoctorInstallationRemoteContracts(
		integrationDoctorRemoteRequest{
			RepositoryRoot: repositoryRoot,
			Repository:     repository,
			Files:          files,
		},
		map[string]string{"NEKO_VERSION": "3.0.4", "NEKO_RELEASE_PLUGIN_VERSION": "4.2.0"},
	) {
		releases[contract.Tag] = append(releases[contract.Tag], contract.RequiredAssets...)
	}
	for tag, assets := range releases {
		releases[tag] = uniqueSortedIntegrationDoctorStrings(assets)
	}
	return releases
}

func TestIntegrationDoctorRemoteAssetsFollowFocusedGoReleaserMatrix(t *testing.T) {
	fixtures := integrationDoctorRemoteReleaseFixtures(t, repositoryInspectionRoot(t).Path())
	for tag, required := range map[string][]string{
		"v3.0.4": {
			"neko-cli_Darwin_arm64.tar.gz",
			"neko-cli_Linux_i386.tar.gz",
			"neko-cli_Windows_i386.zip",
		},
		"plugin-release/v4.2.0": {
			"plugin-release_4.2.0_Darwin_arm64.tar.gz",
			"plugin-release_4.2.0_Linux_i386.tar.gz",
			"plugin-release_4.2.0_Windows_i386.zip",
			"plugin-release_4.2.0_checksums.txt",
		},
	} {
		assets := strings.Join(fixtures[tag], "\n")
		for _, asset := range required {
			if !strings.Contains(assets, asset) {
				t.Errorf("%s assets omit %q: %v", tag, asset, fixtures[tag])
			}
		}
		if strings.Contains(assets, "Darwin_i386") {
			t.Errorf("%s assets retain unsupported Darwin/i386: %v", tag, fixtures[tag])
		}
	}
}

func writeIntegrationDoctorReleaseFixture(writer http.ResponseWriter, tag string, assets []string) {
	encodedAssets := make([]map[string]string, 0, len(assets))
	for _, asset := range assets {
		encodedAssets = append(encodedAssets, map[string]string{"name": asset})
	}
	payload := map[string]any{
		"tag_name": tag, "draft": false, "prerelease": false, "assets": encodedAssets,
	}
	_ = json.NewEncoder(writer).Encode(payload)
}

func uniqueSortedIntegrationDoctorStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	result := make([]string, 0, len(seen))
	for value := range seen {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func integrationDoctorHasCode(diagnostics []integrationDoctorDiagnostic, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

var _ integrationDoctorRemoteInspector = integrationDoctorRecordingRemoteInspector{}
var _ githubReadTokenResolver = integrationDoctorRecordingTokenResolver{}

func TestIntegrationDoctorExplicitRemoteCommandBoundaryUsesFakeServer(t *testing.T) {
	root := repositoryInspectionRoot(t)
	server, requests := newSuccessfulIntegrationDoctorGitHubServer(t, root.Path(), false)
	defer server.Close()
	client := newIntegrationDoctorGitHubReadClientForTest(t, server.URL)
	//nolint:govet // Table fields follow command input then expected scope.
	for _, test := range []struct {
		name  string
		flags map[string]any
		units int
	}{
		{name: "repository wide", flags: map[string]any{"verify-remote": true}, units: 3},
		{name: "unit scoped", flags: map[string]any{"verify-remote": true, "unit": "cli"}, units: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := integrationDoctorCommandHandler{
				inspector: productionIntegrationDoctorUseCaseForTest(t, integrationDoctorGitHubRemoteInspector{
					reader: client,
					tokens: integrationDoctorRecordingTokenResolver{value: "command-boundary-token"},
				}),
				clock: fixedReleaseClock{},
				root:  root,
			}
			response, err := handler.Handle(context.Background(), plugin.Request{
				Command: integrationDoctorCommandName,
				Flags:   test.flags,
			})
			if err != nil || response.ExitCode != 0 {
				t.Fatalf("response=%#v err=%v", response, err)
			}
			remote, ok := response.Data["remote_verification"].(integrationDoctorRemoteSummary)
			if !ok || !remote.Requested || remote.Status != integrationDoctorRemoteComplete {
				t.Fatalf("remote summary=%#v", response.Data["remote_verification"])
			}
			units, ok := response.Data["units"].([]integrationDoctorUnit)
			if !ok || len(units) != test.units {
				t.Fatalf("units=%#v", response.Data["units"])
			}
		})
	}
	for _, request := range requests.snapshot() {
		if request.method != http.MethodGet {
			t.Fatalf("command boundary emitted %s %s", request.method, request.uri)
		}
	}
}

func TestIntegrationDoctorManifestRemoteFlagRoutesThroughCommand(t *testing.T) {
	root := repositoryInspectionRoot(t)
	response, err := HandleDoctorAt(root, plugin.Request{
		Command: integrationDoctorCommandName,
		Flags:   map[string]any{"verify-remote": false},
	})
	if err != nil || response.ExitCode != 0 {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}
