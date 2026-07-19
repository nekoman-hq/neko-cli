package release

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestIntegrationDoctorCanonicalScaffoldIsNotReadyOnlyForConsumerPlaceholder(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", canonicalIntegrationDoctorWorkflow(t))

	response := runIntegrationDoctor(t, root, nil)
	result := integrationDoctorResultFromResponse(t, response)
	if result.Readiness != integrationDoctorNotReady || response.ExitCode != 1 {
		t.Fatalf("readiness=%q exit=%d, want not_ready/1", result.Readiness, response.ExitCode)
	}
	if result.Summary.Errors != 1 || result.Summary.Warnings != 0 || result.Summary.NotVerifiable != 6 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	assertIntegrationDoctorCodes(t, result.Diagnostics, "CONSUMER_PLACEHOLDER_PRESENT")
	if result.Workflows[0].Classification != "canonical_scaffold" {
		t.Fatalf("classification = %q", result.Workflows[0].Classification)
	}
}

func TestIntegrationDoctorCustomEquivalentIsReadyWithExplicitNotVerifiableFacts(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", customIntegrationDoctorWorkflow(t))

	response := runIntegrationDoctor(t, root, nil)
	result := integrationDoctorResultFromResponse(t, response)
	if result.Readiness != integrationDoctorReady || response.ExitCode != 0 {
		t.Fatalf("readiness=%q exit=%d, want ready/0", result.Readiness, response.ExitCode)
	}
	if result.Summary.Errors != 0 || result.Summary.Warnings != 0 || result.Summary.NotVerifiable != 7 {
		t.Fatalf("summary = %#v", result.Summary)
	}
	assertIntegrationDoctorCodes(t, result.Diagnostics,
		"CONSUMER_BUILD_NOT_VERIFIABLE",
		"INSTALLATION_ARTIFACTS_NOT_VERIFIABLE",
		"PUBLICATION_CREDENTIALS_NOT_VERIFIABLE",
		"PUBLICATION_TARGET_NOT_VERIFIABLE",
		"REMOTE_DISPATCH_AUTHORIZATION_NOT_VERIFIABLE",
		"REMOTE_WORKFLOW_NOT_VERIFIABLE",
		"REPOSITORY_VARIABLES_NOT_VERIFIABLE",
	)
}

func TestIntegrationDoctorWarningsRemainSuccessful(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	workflow := bytes.Replace(customIntegrationDoctorWorkflow(t), []byte("  contents: read"), []byte("  contents: write"), 1)
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", workflow)

	response := runIntegrationDoctor(t, root, nil)
	result := integrationDoctorResultFromResponse(t, response)
	if result.Readiness != integrationDoctorReadyWithWarnings || response.ExitCode != 0 {
		t.Fatalf("readiness=%q exit=%d, want ready_with_warnings/0", result.Readiness, response.ExitCode)
	}
	assertIntegrationDoctorCodes(t, result.Diagnostics, "PERMISSIONS_BROAD")
}

func TestIntegrationDoctorWorkflowChecksAreStructuralAndFocused(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		code   string
	}{
		{
			name: "competing tag trigger",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("on:\n  workflow_dispatch:"), []byte("on:\n  push:\n    tags: ['v*']\n  workflow_dispatch:"), 1)
			},
			code: "COMPETING_RELEASE_TRIGGER",
		},
		{
			name: "wrong checkout ref",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("ref: ${{ inputs.release_sha }}"), []byte("ref: main"), 1)
			},
			code: "CHECKOUT_REF_INVALID",
		},
		{
			name: "missing context validator",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("neko release ci-validate-context"), []byte("echo context-validation-removed"), 1)
			},
			code: "CONTEXT_VALIDATOR_MISSING",
		},
		{
			name: "wrong context flag",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("--tag \"$RELEASE_TAG\""), []byte("--tag wrong"), 1)
			},
			code: "CONTEXT_VALIDATOR_FLAG_INVALID",
		},
		{
			name: "missing output integration",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("--github-output-file \"$GITHUB_OUTPUT\""), []byte("--github-output-file result.txt"), 1)
			},
			code: "CONTEXT_OUTPUT_FILE_INVALID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
			writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", test.mutate(customIntegrationDoctorWorkflow(t)))
			result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
			if result.Readiness != integrationDoctorNotReady {
				t.Fatalf("readiness = %q", result.Readiness)
			}
			assertIntegrationDoctorCodes(t, result.Diagnostics, test.code)
		})
	}
}

func TestIntegrationDoctorAllowsUnrelatedTriggersAndOptionalConsumerInput(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	workflow := customIntegrationDoctorWorkflow(t)
	workflow = bytes.Replace(workflow, []byte("on:\n  workflow_dispatch:"), []byte("on:\n  pull_request:\n  push:\n    branches: [main]\n  workflow_dispatch:"), 1)
	workflow = bytes.Replace(workflow, []byte("    inputs:\n"), []byte("    inputs:\n      channel:\n        required: false\n        type: string\n"), 1)
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", workflow)

	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
	if result.Readiness != integrationDoctorReady {
		t.Fatalf("readiness = %q, diagnostics=%#v", result.Readiness, result.Diagnostics)
	}
	assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "COMPETING_RELEASE_TRIGGER")
	assertIntegrationDoctorCodeAbsent(t, result.Diagnostics, "DISPATCH_EXTRA_REQUIRED_INPUT")
}

func TestIntegrationDoctorCoversWorkflowIntegrationBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		code   string
	}{
		{
			name: "required input missing",
			mutate: func(workflow []byte) []byte {
				block := []byte("      tag:\n        description: Neko-created unit tag\n        required: true\n        type: string\n")
				return bytes.Replace(workflow, block, nil, 1)
			},
			code: "DISPATCH_INPUT_MISSING",
		},
		{
			name: "required input optional",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("required: true"), []byte("required: false"), 1)
			},
			code: "DISPATCH_INPUT_NOT_REQUIRED",
		},
		{
			name: "required input wrong type",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("type: string"), []byte("type: boolean"), 1)
			},
			code: "DISPATCH_INPUT_TYPE_INVALID",
		},
		{
			name: "additional required input",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("    inputs:\n"), []byte("    inputs:\n      channel:\n        required: true\n        type: string\n"), 1)
			},
			code: "DISPATCH_EXTRA_REQUIRED_INPUT",
		},
		{
			name: "permissions missing",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("permissions:\n  contents: read\n\n"), nil, 1)
			},
			code: "PERMISSIONS_IMPLICIT",
		},
		{
			name: "concurrency missing",
			mutate: func(workflow []byte) []byte {
				return removeIntegrationDoctorSection(t, workflow, "concurrency:\n", "jobs:\n")
			},
			code: "CONCURRENCY_MISSING",
		},
		{
			name: "checkout missing",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("uses: actions/checkout@v4"), []byte("uses: consumer/example@v1"), 1)
			},
			code: "CHECKOUT_MISSING",
		},
		{
			name: "checkout history shallow",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("fetch-depth: 0"), []byte("fetch-depth: 1"), 1)
			},
			code: "CHECKOUT_HISTORY_INCOMPLETE",
		},
		{
			name: "checkout credentials persist",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("persist-credentials: false"), []byte("persist-credentials: true"), 1)
			},
			code: "CHECKOUT_CREDENTIALS_PERSISTED",
		},
		{
			name: "Neko install missing",
			mutate: func(workflow []byte) []byte {
				return bytes.ReplaceAll(workflow, []byte("install.sh"), []byte("installer"))
			},
			code: "NEKO_INSTALL_MISSING",
		},
		{
			name: "Neko version unpinned",
			mutate: func(workflow []byte) []byte {
				return bytes.ReplaceAll(workflow, []byte("NEKO_VERSION"), []byte("NEKO_CHANNEL"))
			},
			code: "NEKO_VERSION_UNPINNED",
		},
		{
			name: "Release plugin missing",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("neko plugin install release"), []byte("echo release-plugin-removed"), 1)
			},
			code: "RELEASE_PLUGIN_INSTALL_MISSING",
		},
		{
			name: "Release plugin unpinned",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte(" --version \"$NEKO_RELEASE_PLUGIN_VERSION\""), nil, 1)
			},
			code: "RELEASE_PLUGIN_VERSION_UNPINNED",
		},
		{
			name: "validator id missing",
			mutate: func(workflow []byte) []byte {
				return bytes.Replace(workflow, []byte("        id: release-context\n"), nil, 1)
			},
			code: "CONTEXT_STEP_ID_MISSING",
		},
		{
			name: "consumer missing",
			mutate: func(workflow []byte) []byte {
				return removeIntegrationDoctorSection(t, workflow, "      # Consumer-owned extension point.", "integration-doctor-never-present")
			},
			code: "CONSUMER_INTEGRATION_MISSING",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
			workflow := test.mutate(customIntegrationDoctorWorkflow(t))
			writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", workflow)
			result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
			assertIntegrationDoctorCodes(t, result.Diagnostics, test.code)
		})
	}
}

func TestIntegrationDoctorDiagnosesWorkflowFileFailures(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
		result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
		assertIntegrationDoctorCodes(t, result.Diagnostics, "WORKFLOW_MISSING")
	})
	t.Run("malformed", func(t *testing.T) {
		root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
		writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", []byte(": invalid\n"))
		result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
		assertIntegrationDoctorCodes(t, result.Diagnostics, "WORKFLOW_YAML_INVALID")
	})
	t.Run("symlink", func(t *testing.T) {
		root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
		external := filepath.Join(t.TempDir(), "release.yml")
		writeIntegrationDoctorBytes(t, external, customIntegrationDoctorWorkflow(t))
		target := filepath.Join(root.Path(), ".github", "workflows", "release.yml")
		if mkdirErr := os.MkdirAll(filepath.Dir(target), 0755); mkdirErr != nil {
			t.Fatalf("create workflow directory: %v", mkdirErr)
		}
		if linkErr := os.Symlink(external, target); linkErr != nil {
			t.Skipf("symlink unavailable: %v", linkErr)
		}
		result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
		assertIntegrationDoctorCodes(t, result.Diagnostics, "WORKFLOW_PATH_ESCAPE")
	})
}

func TestIntegrationDoctorDiagnosesMissingMalformedAndConflictingSources(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*testing.T, string)
		codes []string
	}{
		{name: "missing pair", codes: []string{"V2_CONFIG_MISSING", "V2_STATE_MISSING"}},
		{
			name: "config only",
			setup: func(t *testing.T, root string) {
				writeIntegrationDoctorBytes(t, releaseconfig.V2ConfigPath(root), []byte(`{"schemaVersion":2,"units":[]}`))
			},
			codes: []string{"V2_STATE_MISSING"},
		},
		{
			name: "state only",
			setup: func(t *testing.T, root string) {
				writeIntegrationDoctorBytes(t, releaseconfig.V2StatePath(root), []byte(`{"schemaVersion":2,"units":{}}`))
			},
			codes: []string{"V2_CONFIG_MISSING"},
		},
		{
			name: "malformed config",
			setup: func(t *testing.T, root string) {
				writeIntegrationDoctorBytes(t, releaseconfig.V2ConfigPath(root), []byte(`{"schemaVersion":2,`))
				writeIntegrationDoctorBytes(t, releaseconfig.V2StatePath(root), []byte(`{"schemaVersion":2,"units":{}}`))
			},
			codes: []string{"V2_CONFIG_INVALID"},
		},
		{
			name: "malformed state",
			setup: func(t *testing.T, root string) {
				writeIntegrationDoctorBytes(t, releaseconfig.V2ConfigPath(root), []byte(`{"schemaVersion":2,"units":[]}`))
				writeIntegrationDoctorBytes(t, releaseconfig.V2StatePath(root), []byte(`{"schemaVersion":2,`))
			},
			codes: []string{"V2_STATE_INVALID"},
		},
		{
			name: "V1 only",
			setup: func(t *testing.T, root string) {
				writeIntegrationDoctorBytes(t, filepath.Join(root, releaseconfig.V1FileName), []byte("{}")) //nolint:staticcheck
			},
			codes: []string{"V1_SOURCE_UNSUPPORTED"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootPath := t.TempDir()
			if mkdirErr := os.Mkdir(filepath.Join(rootPath, ".git"), 0755); mkdirErr != nil {
				t.Fatalf("create git marker: %v", mkdirErr)
			}
			if test.setup != nil {
				test.setup(t, rootPath)
			}
			root, err := workspace.ResolveInspectionRepositoryRoot(rootPath)
			if err != nil {
				t.Fatalf("resolve inspection root: %v", err)
			}
			result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
			if result.Readiness != integrationDoctorNotReady {
				t.Fatalf("readiness = %q", result.Readiness)
			}
			assertIntegrationDoctorCodes(t, result.Diagnostics, test.codes...)
		})
	}
}

func TestIntegrationDoctorSourceInspectionFailureIsNotReady(t *testing.T) {
	inspection := inspectIntegrationDoctorSource("/unavailable", integrationDoctorSourceSnapshot{
		InspectionErr: errors.New("read denied"),
	})
	result := &integrationDoctorResult{Diagnostics: inspection.Diagnostics}
	finalizeIntegrationDoctorResult(result)
	if inspection.Repository != nil || result.Readiness != integrationDoctorNotReady {
		t.Fatalf("inspection=%#v result=%#v", inspection, result)
	}
	assertIntegrationDoctorCodes(t, result.Diagnostics, "SOURCE_INSPECTION_FAILED")
}

func TestIntegrationDoctorDiagnosesAlignmentRecoveryAndMixedSources(t *testing.T) {
	t.Run("alignment", func(t *testing.T) {
		root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
		writeWorkflowScaffoldJSON(t, releaseconfig.V2StatePath(root.Path()), releaseconfig.V2ReleaseState{
			SchemaVersion: 2,
			Units:         map[string]releaseconfig.V2UnitState{"web": {Version: "1.2.3"}},
		})
		assertIntegrationDoctorCodes(t, integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil)).Diagnostics, "V2_CONFIG_STATE_MISMATCH")
	})
	t.Run("recovery", func(t *testing.T) {
		root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
		writeIntegrationDoctorBytes(t, releaseconfig.V2PairRecoveryPath(root.Path()), []byte("{}\n"))
		assertIntegrationDoctorCodes(t, integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil)).Diagnostics, "V2_RECOVERY_BLOCKED")
	})
	t.Run("mixed", func(t *testing.T) {
		root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
		writeIntegrationDoctorBytes(t, filepath.Join(root.Path(), releaseconfig.V1FileName), []byte("{}\n")) //nolint:staticcheck
		assertIntegrationDoctorCodes(t, integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil)).Diagnostics, "MIXED_RELEASE_SOURCES")
	})
}

func TestIntegrationDoctorClassifiesStrictV2StructureFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*releaseconfig.V2ReleaseConfig, *releaseconfig.V2ReleaseState)
		code   string
	}{
		{
			name: "schema",
			mutate: func(config *releaseconfig.V2ReleaseConfig, _ *releaseconfig.V2ReleaseState) {
				config.SchemaVersion = 3
			},
			code: "V2_SCHEMA_VERSION_INVALID",
		},
		{
			name: "version",
			mutate: func(_ *releaseconfig.V2ReleaseConfig, state *releaseconfig.V2ReleaseState) {
				state.Units["api"] = releaseconfig.V2UnitState{Version: "latest"}
			},
			code: "UNIT_VERSION_INVALID",
		},
		{
			name: "executor",
			mutate: func(config *releaseconfig.V2ReleaseConfig, _ *releaseconfig.V2ReleaseState) {
				config.Units[0].Executor.Type = releaseconfig.ExecutorType("custom")
			},
			code: "EXECUTOR_INVALID",
		},
		{
			name: "delivery",
			mutate: func(config *releaseconfig.V2ReleaseConfig, _ *releaseconfig.V2ReleaseState) {
				config.Units[0].Executor.Delivery = releaseconfig.DeliveryLocal
			},
			code: "DELIVERY_INVALID",
		},
		{
			name: "workflow path",
			mutate: func(config *releaseconfig.V2ReleaseConfig, _ *releaseconfig.V2ReleaseState) {
				config.Units[0].Executor.Workflow = "../release.yml"
			},
			code: "WORKFLOW_CONFIGURATION_INVALID",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
			config, configErr := releaseconfig.LoadV2Config(releaseconfig.V2ConfigPath(root.Path()))
			if configErr != nil {
				t.Fatalf("load config: %v", configErr)
			}
			state, stateErr := releaseconfig.LoadV2State(releaseconfig.V2StatePath(root.Path()))
			if stateErr != nil {
				t.Fatalf("load state: %v", stateErr)
			}
			test.mutate(config, state)
			writeWorkflowScaffoldJSON(t, releaseconfig.V2ConfigPath(root.Path()), config)
			writeWorkflowScaffoldJSON(t, releaseconfig.V2StatePath(root.Path()), state)
			result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
			assertIntegrationDoctorCodes(t, result.Diagnostics, test.code)
		})
	}
}

func TestIntegrationDoctorDiagnosesDuplicateTagPrefixes(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{
		"api": ".github/workflows/release-api.yml",
		"web": ".github/workflows/release-web.yml",
	})
	config, err := releaseconfig.LoadV2Config(releaseconfig.V2ConfigPath(root.Path()))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	config.Units[1].TagPrefix = config.Units[0].TagPrefix
	writeWorkflowScaffoldJSON(t, releaseconfig.V2ConfigPath(root.Path()), config)
	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, nil))
	assertIntegrationDoctorCodes(t, result.Diagnostics, "TAG_PREFIX_CONFLICT")
}

func TestIntegrationDoctorUnitSelectionRetainsSharedWorkflowScope(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{
		"api": ".github/workflows/release.yml",
		"web": ".github/workflows/release.yml",
	})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", customIntegrationDoctorWorkflow(t))

	result := integrationDoctorResultFromResponse(t, runIntegrationDoctor(t, root, map[string]any{"unit": "api"}))
	if len(result.Units) != 1 || result.Units[0].ID != "api" {
		t.Fatalf("units = %#v", result.Units)
	}
	if len(result.Workflows) != 1 || !reflect.DeepEqual(result.Workflows[0].Units, []string{"api", "web"}) {
		t.Fatalf("workflow scope = %#v", result.Workflows)
	}
}

func TestIntegrationDoctorIsTokenFreeAndDoesNotMutateInspectedFiles(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	workflowPath := writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", customIntegrationDoctorWorkflow(t))
	paths := []string{releaseconfig.V2ConfigPath(root.Path()), releaseconfig.V2StatePath(root.Path()), workflowPath}
	preserved := time.Unix(1_710_000_000, 0)
	before := map[string]integrationDoctorFileSnapshot{}
	for _, path := range paths {
		if timeErr := os.Chtimes(path, preserved, preserved); timeErr != nil {
			t.Fatalf("set preserved time: %v", timeErr)
		}
		before[path] = snapshotIntegrationDoctorFile(t, path)
	}
	t.Setenv("GITHUB_TOKEN", "integration-doctor-secret-sentinel")

	response := runIntegrationDoctor(t, root, nil)
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if bytes.Contains(encoded, []byte("integration-doctor-secret-sentinel")) {
		t.Fatal("doctor response leaked ambient token")
	}
	for _, path := range paths {
		if got := snapshotIntegrationDoctorFile(t, path); !reflect.DeepEqual(got, before[path]) {
			t.Fatalf("doctor mutated %s: got=%#v want=%#v", path, got, before[path])
		}
	}
}

func TestIntegrationDoctorRejectsInvalidUnitFlagWithoutInspection(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	response := runIntegrationDoctor(t, root, map[string]any{"unit": true})
	if response.Status != "error" || response.Error == nil || response.Error.Code != "INVALID_DOCTOR_REQUEST" || response.ExitCode != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestHandleDoctorResolvesNestedExplicitInspectionRoot(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", customIntegrationDoctorWorkflow(t))
	nested := filepath.Join(root.Path(), "services", "api")
	if mkdirErr := os.MkdirAll(nested, 0755); mkdirErr != nil {
		t.Fatalf("create nested directory: %v", mkdirErr)
	}
	other := t.TempDir()
	t.Chdir(other)

	response, err := HandleDoctor(plugin.Request{
		Command: integrationDoctorCommandName,
		Context: plugin.Context{WorkingDir: nested},
	})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	result := integrationDoctorResultFromResponse(t, response)
	if len(result.Units) != 1 || result.Units[0].ID != "api" {
		t.Fatalf("units = %#v", result.Units)
	}
	got, getErr := os.Getwd()
	gotInfo, gotStatErr := os.Stat(got)
	otherInfo, otherStatErr := os.Stat(other)
	if getErr != nil || gotStatErr != nil || otherStatErr != nil || !os.SameFile(gotInfo, otherInfo) {
		t.Fatalf("doctor changed process cwd: got=%q err=%v", got, getErr)
	}
}

//nolint:govet // Logical file metadata order keeps mutation assertions readable.
type integrationDoctorFileSnapshot struct {
	Content []byte
	Mode    os.FileMode
	ModTime time.Time
}

func newIntegrationDoctorRepository(t *testing.T, units map[string]string) workspace.RepositoryRoot {
	t.Helper()
	return newWorkflowScaffoldRepository(t, units)
}

func canonicalIntegrationDoctorWorkflow(t *testing.T) []byte {
	t.Helper()
	workflow, err := RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render canonical workflow: %v", err)
	}
	return workflow
}

func customIntegrationDoctorWorkflow(t *testing.T) []byte {
	t.Helper()
	workflow := canonicalIntegrationDoctorWorkflow(t)
	placeholder := []byte("          echo \"::error::" + generatedConsumerPlaceholder + "\" >&2\n          exit 1")
	consumer := []byte("          echo \"publish $RELEASE_TAG\"")
	custom := bytes.Replace(workflow, placeholder, consumer, 1)
	if bytes.Equal(custom, workflow) {
		t.Fatal("canonical consumer placeholder was not replaced")
	}
	return custom
}

func writeIntegrationDoctorWorkflow(t *testing.T, root workspace.RepositoryRoot, relativePath string, content []byte) string {
	t.Helper()
	path := filepath.Join(root.Path(), filepath.FromSlash(relativePath))
	writeIntegrationDoctorBytes(t, path, content)
	return path
}

func writeIntegrationDoctorBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0755); mkdirErr != nil {
		t.Fatalf("create parent for %s: %v", path, mkdirErr)
	}
	if writeErr := os.WriteFile(path, content, 0644); writeErr != nil {
		t.Fatalf("write %s: %v", path, writeErr)
	}
}

func runIntegrationDoctor(t *testing.T, root workspace.RepositoryRoot, flags map[string]any) *plugin.Response {
	t.Helper()
	response, err := HandleDoctorAt(root, plugin.Request{Command: integrationDoctorCommandName, Flags: flags})
	if err != nil {
		t.Fatalf("HandleDoctorAt: %v", err)
	}
	return response
}

func integrationDoctorResultFromResponse(t *testing.T, response *plugin.Response) integrationDoctorResult {
	t.Helper()
	result := integrationDoctorResult{}
	var ok bool
	result.Readiness, ok = response.Data["readiness"].(integrationDoctorReadiness)
	if !ok {
		t.Fatalf("readiness type = %T", response.Data["readiness"])
	}
	result.Summary, ok = response.Data["summary"].(integrationDoctorSummary)
	if !ok {
		t.Fatalf("summary type = %T", response.Data["summary"])
	}
	result.Units, ok = response.Data["units"].([]integrationDoctorUnit)
	if !ok {
		t.Fatalf("units type = %T", response.Data["units"])
	}
	result.Workflows, ok = response.Data["workflows"].([]integrationDoctorWorkflow)
	if !ok {
		t.Fatalf("workflows type = %T", response.Data["workflows"])
	}
	result.Verifications, ok = response.Data["verifications"].([]integrationDoctorVerification)
	if !ok {
		t.Fatalf("verifications type = %T", response.Data["verifications"])
	}
	result.Diagnostics, ok = response.Data["diagnostics"].([]integrationDoctorDiagnostic)
	if !ok {
		t.Fatalf("diagnostics type = %T", response.Data["diagnostics"])
	}
	return result
}

func assertIntegrationDoctorCodes(t *testing.T, diagnostics []integrationDoctorDiagnostic, codes ...string) {
	t.Helper()
	for _, code := range codes {
		found := false
		for _, diagnostic := range diagnostics {
			if diagnostic.Code == code {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("diagnostic %s missing from %#v", code, diagnostics)
		}
	}
}

func assertIntegrationDoctorCodeAbsent(t *testing.T, diagnostics []integrationDoctorDiagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			t.Fatalf("unexpected diagnostic %s in %#v", code, diagnostics)
		}
	}
}

func snapshotIntegrationDoctorFile(t *testing.T, path string) integrationDoctorFileSnapshot {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return integrationDoctorFileSnapshot{Content: content, Mode: info.Mode(), ModTime: info.ModTime()}
}

func removeIntegrationDoctorSection(t *testing.T, content []byte, startMarker, endMarker string) []byte {
	t.Helper()
	start := bytes.Index(content, []byte(startMarker))
	if start < 0 {
		t.Fatalf("start marker %q missing", startMarker)
	}
	end := len(content)
	if endMarker != "integration-doctor-never-present" {
		endOffset := bytes.Index(content[start:], []byte(endMarker))
		if endOffset < 0 {
			t.Fatalf("end marker %q missing", endMarker)
		}
		end = start + endOffset
	}
	return append(append([]byte(nil), content[:start]...), content[end:]...)
}

func TestIntegrationDoctorDiagnosticOrderingAndReadinessAreClosed(t *testing.T) {
	result := &integrationDoctorResult{Diagnostics: []integrationDoctorDiagnostic{
		newIntegrationDoctorDiagnostic(integrationDoctorNotVerifiable, "remote", "REMOTE", "remote", "review"),
		newIntegrationDoctorDiagnostic(integrationDoctorRecommendation, "workflow", "RECOMMEND", "recommend", "review"),
		newIntegrationDoctorDiagnostic(integrationDoctorWarning, "workflow", "WARN", "warn", "repair"),
		newIntegrationDoctorDiagnostic(integrationDoctorError, "source", "ERROR", "error", "repair"),
	}}
	finalizeIntegrationDoctorResult(result)
	if result.Readiness != integrationDoctorNotReady || result.Diagnostics[0].Code != "ERROR" || result.Diagnostics[3].Code != "REMOTE" {
		t.Fatalf("result = %#v", result)
	}
	result.Diagnostics = result.Diagnostics[1:]
	finalizeIntegrationDoctorResult(result)
	if result.Readiness != integrationDoctorReadyWithWarnings {
		t.Fatalf("warning readiness = %q", result.Readiness)
	}
	result.Diagnostics = result.Diagnostics[1:]
	finalizeIntegrationDoctorResult(result)
	if result.Readiness != integrationDoctorReady {
		t.Fatalf("recommendation/not-verifiable readiness = %q", result.Readiness)
	}
}

func TestIntegrationDoctorWorkflowParserTreatsOnAsStructuralKey(t *testing.T) {
	workflow := customIntegrationDoctorWorkflow(t)
	snapshot := filesystemIntegrationDoctorWorkflowReader{}
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", workflow)
	parsed := snapshot.Read(root.Path(), ".github/workflows/release.yml")
	if parsed.Document == nil || workflowMappingValue(workflowDocumentRoot(parsed.Document), "on") == nil {
		t.Fatal("YAML parser did not preserve the GitHub Actions on mapping")
	}
}

func TestIntegrationDoctorResultJSONHasStableSchemaWithoutPresentationMetadata(t *testing.T) {
	root := newIntegrationDoctorRepository(t, map[string]string{"api": ".github/workflows/release.yml"})
	writeIntegrationDoctorWorkflow(t, root, ".github/workflows/release.yml", customIntegrationDoctorWorkflow(t))
	response := runIntegrationDoctor(t, root, nil)

	encoded, err := json.Marshal(response.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	for _, key := range []string{`"readiness"`, `"summary"`, `"units"`, `"workflows"`, `"verifications"`, `"diagnostics"`} {
		if !strings.Contains(string(encoded), key) {
			t.Fatalf("JSON omitted %s: %s", key, encoded)
		}
	}
	if strings.Contains(string(encoded), "human_properties") || strings.Contains(string(encoded), "renderer_hint") {
		t.Fatalf("data leaked presentation metadata: %s", encoded)
	}
}
