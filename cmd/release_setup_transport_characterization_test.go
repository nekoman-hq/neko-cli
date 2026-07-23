package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	initcmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/init"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/migrate"
	releasecmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

const releaseSetupHelperEnvironment = "NEKO_RELEASE_SETUP_HELPER"

func TestReleaseSetupPluginHelperProcess(t *testing.T) {
	if os.Getenv(releaseSetupHelperEnvironment) != "1" {
		return
	}

	var request plugin.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		t.Fatalf("decode plugin request: %v", err)
	}
	request.Context.WorkingDir = os.Getenv(releaseReadonlyRootEnvironment)
	log.Verbose = request.Context.Verbose

	var response *plugin.Response
	var executeErr error
	if request.Command == "migrate" {
		response, executeErr = migrate.HandleMigrate(request)
	} else {
		root, resolveErr := workspace.ResolveRepositoryRoot(request.Context.WorkingDir)
		if resolveErr != nil {
			t.Fatalf("resolve setup repository: %v", resolveErr)
		}
		switch request.Command {
		case "init":
			response, executeErr = initcmd.HandleInitAt(root, request)
		case "unit-add":
			response, executeErr = initcmd.HandleUnitAddAt(root, request)
		case "github-workflow-init":
			response, executeErr = releasecmd.HandleGitHubWorkflowInitAt(root, request)
		default:
			t.Fatalf("unexpected setup helper command %q", request.Command)
		}
	}
	if executeErr != nil {
		t.Fatalf("execute %s: %v", request.Command, executeErr)
	}
	if encodeErr := json.NewEncoder(os.Stdout).Encode(response); encodeErr != nil {
		t.Fatalf("encode %s response: %v", request.Command, encodeErr)
	}
	os.Exit(0)
}

func TestReleaseSetupCommandsPreserveDomainJSONAcrossGlobalModes(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)
	tests := []struct {
		fixture func(*testing.T) (string, []string)
		name    string
		command string
	}{
		{name: "init", command: "init", fixture: newReleaseSetupInitRepository},
		{name: "unit add", command: "unit-add", fixture: newReleaseSetupUnitAddRepository},
		{name: "migrate dry run", command: "migrate", fixture: newReleaseSetupMigrationRepository},
		{name: "migrate actual", command: "migrate", fixture: newReleaseSetupMigrationActualRepository},
		{name: "workflow dry run", command: "github-workflow-init", fixture: newReleaseSetupWorkflowRepository},
		{name: "workflow actual", command: "github-workflow-init", fixture: newReleaseSetupWorkflowActualRepository},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plainRoot, flags := test.fixture(t)
			plain, plainErr := executeReleaseReadonlyCommand(
				t, manifest, plainRoot, test.command, flags, releaseReadonlyMode{format: "json"},
			)
			describeRoot, flags := test.fixture(t)
			described, describedErr := executeReleaseReadonlyCommand(
				t, manifest, describeRoot, test.command, flags, releaseReadonlyMode{format: "json", describe: true},
			)
			verboseRoot, flags := test.fixture(t)
			verboseOutput, verboseErr := executeReleaseReadonlyCommand(
				t, manifest, verboseRoot, test.command, flags, releaseReadonlyMode{format: "json", verbose: true},
			)
			if !samePluginExit(plainErr, describedErr) || !samePluginExit(plainErr, verboseErr) {
				t.Fatalf("global modes changed exit behavior: plain=%v describe=%v verbose=%v", plainErr, describedErr, verboseErr)
			}
			plainResponse := decodeReleaseReadonlyPublicResponse(t, plain)
			plainResponse.Data = normalizeReleaseSetupData(t, plainResponse.Data, plainRoot)
			describeResponse := decodeReleaseReadonlyPublicResponse(t, described)
			describeResponse.Data = normalizeReleaseSetupData(t, describeResponse.Data, describeRoot)
			verboseResponse := decodeReleaseReadonlyPublicResponse(t, verboseOutput)
			verboseResponse.Data = normalizeReleaseSetupData(t, verboseResponse.Data, verboseRoot)
			if !reflect.DeepEqual(plainResponse.Data, describeResponse.Data) {
				t.Fatalf("describe changed %s domain data\nplain=%#v\ndescribe=%#v", test.command, plainResponse.Data, describeResponse.Data)
			}
			if !reflect.DeepEqual(plainResponse.Data, verboseResponse.Data) {
				t.Fatalf("verbose changed %s domain data\nplain=%#v\nverbose=%#v", test.command, plainResponse.Data, verboseResponse.Data)
			}
			for _, output := range []string{plain, described, verboseOutput} {
				for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b[", "setup-transport-secret"} {
					if strings.Contains(output, forbidden) {
						t.Fatalf("%s public JSON contains %q:\n%s", test.command, forbidden, output)
					}
				}
			}
		})
	}
}

func TestReleaseInitAndUnitAddCorePresentationContracts(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)
	tests := []struct {
		name            string
		command         string
		fixture         func(*testing.T) (string, []string)
		defaultFacts    []string
		describeFacts   []string
		verbosePhases   []string
		defaultNotFacts []string
	}{
		{
			name: "init", command: "init", fixture: newReleaseSetupInitRepository,
			defaultFacts: []string{
				"Release Initialization", "Initialized unit", "api", "Version", "1.2.3",
				"Configuration", ".neko/release.config.json", "Next action",
			},
			describeFacts: []string{
				"Resolved Configuration", "Artifact Write Plan", "Validation Facts", "Limitations",
			},
			verbosePhases: []string{
				"Validating initialization command inputs", "Inspecting repository initialization state",
				"Resolving V2 release unit configuration", "Writing V2 configuration and state",
				"Initialization completed successfully",
			},
			defaultNotFacts: []string{"Resolved Configuration", "Artifact Write Plan", "Validation Facts", "Limitations"},
		},
		{
			name: "unit add", command: "unit-add", fixture: newReleaseSetupUnitAddRepository,
			defaultFacts: []string{
				"Release Unit Added", "Added unit", "web", "Version", "2.3.4",
				"Configuration", ".neko/release.config.json", "Next action",
			},
			describeFacts: []string{
				"Resolved Unit", "Existing Unit Comparison", "Artifact Write Plan", "Validation Facts", "Limitations",
			},
			verbosePhases: []string{
				"Validating release unit inputs", "Inspecting existing V2 configuration and state",
				"Reading existing V2 configuration and state", "Checking duplicate unit identity",
				"Writing updated V2 configuration and state", "Release unit append completed",
			},
			defaultNotFacts: []string{"Resolved Unit", "Existing Unit Comparison", "Artifact Write Plan", "Validation Facts", "Limitations"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defaultRoot, flags := test.fixture(t)
			defaultOutput, defaultErr := executeReleaseReadonlyCommand(
				t, manifest, defaultRoot, test.command, flags, releaseReadonlyMode{},
			)
			describeRoot, flags := test.fixture(t)
			describeOutput, describeErr := executeReleaseReadonlyCommand(
				t, manifest, describeRoot, test.command, flags, releaseReadonlyMode{describe: true},
			)
			verboseRoot, flags := test.fixture(t)
			verboseOutput, verboseErr := executeReleaseReadonlyCommand(
				t, manifest, verboseRoot, test.command, flags, releaseReadonlyMode{verbose: true},
			)
			combinedRoot, flags := test.fixture(t)
			combinedOutput, combinedErr := executeReleaseReadonlyCommand(
				t, manifest, combinedRoot, test.command, flags, releaseReadonlyMode{describe: true, verbose: true},
			)
			if defaultErr != nil || describeErr != nil || verboseErr != nil || combinedErr != nil {
				t.Fatalf("setup exits: default=%v describe=%v verbose=%v combined=%v", defaultErr, describeErr, verboseErr, combinedErr)
			}
			for _, want := range test.defaultFacts {
				if !strings.Contains(defaultOutput, want) {
					t.Fatalf("default omitted %q:\n%s", want, defaultOutput)
				}
			}
			for _, hidden := range test.defaultNotFacts {
				if strings.Contains(defaultOutput, hidden) {
					t.Fatalf("default exposed %q:\n%s", hidden, defaultOutput)
				}
			}
			for _, want := range test.describeFacts {
				if !strings.Contains(describeOutput, want) || !strings.Contains(combinedOutput, want) {
					t.Fatalf("describe modes omitted %q:\ndescribe:\n%s\ncombined:\n%s", want, describeOutput, combinedOutput)
				}
			}
			for _, want := range test.verbosePhases {
				if !strings.Contains(verboseOutput, want) || !strings.Contains(combinedOutput, want) {
					t.Fatalf("verbose modes omitted %q:\nverbose:\n%s\ncombined:\n%s", want, verboseOutput, combinedOutput)
				}
			}
			outputs := []struct {
				root   string
				output string
			}{
				{root: defaultRoot, output: defaultOutput},
				{root: describeRoot, output: describeOutput},
				{root: verboseRoot, output: verboseOutput},
				{root: combinedRoot, output: combinedOutput},
			}
			for _, result := range outputs {
				for _, forbidden := range []string{result.root, "\x1b[", "setup-transport-secret", "Authorization", "Bearer"} {
					if strings.Contains(result.output, forbidden) {
						t.Fatalf("%s output contains %q:\n%s", test.command, forbidden, result.output)
					}
				}
			}
		})
	}
}

func TestReleaseInitAndUnitAddConflictsRemainActionableAndWriteNothing(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)

	initRoot, initFlags := newReleaseSetupInitRepository(t)
	existingConfig := filepath.Join(initRoot, ".neko", "release.config.json")
	existingState := filepath.Join(initRoot, ".neko", "release.state.json")
	writeReleaseSetupFile(t, existingConfig, "existing config\n")
	writeReleaseSetupFile(t, existingState, "existing state\n")
	initOutput, initErr := executeReleaseReadonlyCommand(
		t, manifest, initRoot, "init", initFlags, releaseReadonlyMode{},
	)
	if initErr != nil {
		t.Fatalf("init conflict changed characterized Core exit behavior: %v", initErr)
	}
	for _, want := range []string{"CONFIG_EXISTS", "Conflict", "Force applicable", "Yes", "--force"} {
		if !strings.Contains(initOutput, want) {
			t.Fatalf("init conflict omitted %q:\n%s", want, initOutput)
		}
	}
	if content, err := os.ReadFile(existingConfig); err != nil || string(content) != "existing config\n" {
		t.Fatalf("init conflict changed existing config: content=%q err=%v", content, err)
	}
	if content, err := os.ReadFile(existingState); err != nil || string(content) != "existing state\n" {
		t.Fatalf("init conflict changed existing state: content=%q err=%v", content, err)
	}

	unitRoot, _ := newReleaseSetupUnitAddRepository(t)
	duplicateFlags := []string{
		"--unit", "api", "--display-name", "API", "--version", "9.9.9",
		"--executor", "goreleaser", "--delivery", "github-actions",
		"--workflow", ".github/workflows/release-api.yml", "--tag-prefix", "api/v",
		"--working-directory", ".", "--paths", "services/api/**",
	}
	configBefore, err := os.ReadFile(filepath.Join(unitRoot, ".neko", "release.config.json"))
	if err != nil {
		t.Fatalf("read unit config before duplicate: %v", err)
	}
	stateBefore, err := os.ReadFile(filepath.Join(unitRoot, ".neko", "release.state.json"))
	if err != nil {
		t.Fatalf("read unit state before duplicate: %v", err)
	}
	unitOutput, unitErr := executeReleaseReadonlyCommand(
		t, manifest, unitRoot, "unit-add", duplicateFlags, releaseReadonlyMode{},
	)
	if unitErr != nil {
		t.Fatalf("duplicate conflict changed characterized Core exit behavior: %v", unitErr)
	}
	for _, want := range []string{"DUPLICATE_UNIT", "Conflict", "Duplicate unit", "Force applicable", "No", "Choose a new unit id"} {
		if !strings.Contains(unitOutput, want) {
			t.Fatalf("duplicate conflict omitted %q:\n%s", want, unitOutput)
		}
	}
	configAfter, _ := os.ReadFile(filepath.Join(unitRoot, ".neko", "release.config.json"))
	stateAfter, _ := os.ReadFile(filepath.Join(unitRoot, ".neko", "release.state.json"))
	if !reflect.DeepEqual(configBefore, configAfter) || !reflect.DeepEqual(stateBefore, stateAfter) {
		t.Fatal("duplicate unit changed the V2 config/state pair")
	}
}

func TestReleaseMigrateCorePresentationAndMutationContracts(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)

	defaultRoot, flags := newReleaseSetupMigrationRepository(t)
	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, defaultRoot, "migrate", flags, releaseReadonlyMode{},
	)
	describeRoot, flags := newReleaseSetupMigrationRepository(t)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, describeRoot, "migrate", flags, releaseReadonlyMode{describe: true},
	)
	verboseRoot, flags := newReleaseSetupMigrationRepository(t)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, verboseRoot, "migrate", flags, releaseReadonlyMode{verbose: true},
	)
	combinedRoot, flags := newReleaseSetupMigrationRepository(t)
	combinedOutput, combinedErr := executeReleaseReadonlyCommand(
		t, manifest, combinedRoot, "migrate", flags, releaseReadonlyMode{describe: true, verbose: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil || combinedErr != nil {
		t.Fatalf("migrate exits: default=%v describe=%v verbose=%v combined=%v", defaultErr, describeErr, verboseErr, combinedErr)
	}
	for _, want := range []string{
		"Release Migration", "Dry run", "Yes", "Source contract", "V1", "Destination contract", "V2",
		"Planned actions", "Configuration", ".neko/release.config.json", "Archive", ".release.neko.json.v1.bak",
	} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("migrate default omitted %q:\n%s", want, defaultOutput)
		}
	}
	for _, hidden := range []string{"Source Facts", "Resolved V2 Configuration", "Ordered Migration Plan", "Validation Facts", "Limitations"} {
		if strings.Contains(defaultOutput, hidden) ||
			!strings.Contains(describeOutput, hidden) || !strings.Contains(combinedOutput, hidden) {
			t.Fatalf("migrate describe visibility for %q is incorrect:\ndefault:\n%s\ndescribe:\n%s\ncombined:\n%s", hidden, defaultOutput, describeOutput, combinedOutput)
		}
	}
	for _, want := range []string{
		"Resolving migration repository root", "Locating V1 release configuration",
		"Validating V1 migration source", "Deriving V2 release configuration",
		"Planning archive and migration journal actions", "Dry-run selected; no migration files written",
	} {
		if !strings.Contains(verboseOutput, want) || !strings.Contains(combinedOutput, want) {
			t.Fatalf("migrate verbose modes omitted %q:\nverbose:\n%s\ncombined:\n%s", want, verboseOutput, combinedOutput)
		}
	}
	outputs := []struct {
		root   string
		output string
	}{
		{root: defaultRoot, output: defaultOutput},
		{root: describeRoot, output: describeOutput},
		{root: verboseRoot, output: verboseOutput},
		{root: combinedRoot, output: combinedOutput},
	}
	for _, result := range outputs {
		for _, forbidden := range []string{result.root, "\x1b[", "setup-transport-secret", "Authorization", "Bearer", `"schemaVersion"`} {
			if strings.Contains(result.output, forbidden) {
				t.Fatalf("migrate human output contains %q:\n%s", forbidden, result.output)
			}
		}
		if !existsReleaseSetupFile(filepath.Join(result.root, ".release.neko.json")) ||
			existsReleaseSetupFile(filepath.Join(result.root, ".neko", "release.config.json")) ||
			existsReleaseSetupFile(filepath.Join(result.root, ".neko", "release.state.json")) ||
			existsReleaseSetupFile(filepath.Join(result.root, ".release.neko.json.v1.bak")) ||
			existsReleaseSetupFile(filepath.Join(result.root, ".neko", "release.migration.json")) {
			t.Fatalf("migrate dry-run changed fixture %s", result.root)
		}
	}

	actualRoot, actualFlags := newReleaseSetupMigrationActualRepository(t)
	unrelated := filepath.Join(actualRoot, "unrelated.txt")
	writeReleaseSetupFile(t, unrelated, "unchanged\n")
	actualOutput, actualErr := executeReleaseReadonlyCommand(
		t, manifest, actualRoot, "migrate", actualFlags, releaseReadonlyMode{verbose: true},
	)
	if actualErr != nil {
		t.Fatalf("actual migrate exit: %v\n%s", actualErr, actualOutput)
	}
	for _, want := range []string{
		"Migration completed", "Writing V2 configuration and state", "V2 configuration and state written",
		"Archiving legacy V1 configuration", "Legacy V1 configuration archived", "Migration execution completed",
	} {
		if !strings.Contains(actualOutput, want) {
			t.Fatalf("actual migrate omitted %q:\n%s", want, actualOutput)
		}
	}
	if existsReleaseSetupFile(filepath.Join(actualRoot, ".release.neko.json")) ||
		!existsReleaseSetupFile(filepath.Join(actualRoot, ".neko", "release.config.json")) ||
		!existsReleaseSetupFile(filepath.Join(actualRoot, ".neko", "release.state.json")) ||
		!existsReleaseSetupFile(filepath.Join(actualRoot, ".release.neko.json.v1.bak")) ||
		existsReleaseSetupFile(filepath.Join(actualRoot, ".neko", "release.migration.json")) {
		t.Fatal("actual migration did not create/archive exactly the expected artifacts")
	}
	if content, err := os.ReadFile(unrelated); err != nil || string(content) != "unchanged\n" {
		t.Fatalf("actual migration changed unrelated file: content=%q err=%v", content, err)
	}
}

func TestReleaseMigrateConflictIsActionableAndWriteFree(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)
	root, _ := newReleaseSetupMigrationRepository(t)
	writeReleaseSetupFile(t, filepath.Join(root, ".neko", "release.config.json"), releaseSetupMigratedConfig)
	writeReleaseSetupFile(t, filepath.Join(root, ".neko", "release.state.json"), releaseSetupMigratedState)
	before := snapshotReleaseSetupFiles(t, root)

	output, err := executeReleaseReadonlyCommand(
		t, manifest, root, "migrate", nil, releaseReadonlyMode{},
	)
	if err != nil {
		t.Fatalf("migrate conflict changed characterized Core exit behavior: %v", err)
	}
	for _, want := range []string{
		"MIGRATION_FAILED", "Migration Blocked", "V1/V2 source conflict", "Refused",
		"No files written", "migration will not overwrite V2",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("migrate conflict omitted %q:\n%s", want, output)
		}
	}
	for _, forbidden := range []string{root, "\x1b[", "setup-transport-secret", "Authorization", "Bearer"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("migrate conflict output contains %q:\n%s", forbidden, output)
		}
	}
	after := snapshotReleaseSetupFiles(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("migrate conflict changed fixture files")
	}
}

func TestReleaseWorkflowInitCorePresentationAndIdempotencyContracts(t *testing.T) {
	manifest := installReleaseSetupHelperPlugin(t)

	defaultRoot, flags := newReleaseSetupWorkflowMissingRepository(t)
	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, defaultRoot, "github-workflow-init", flags, releaseReadonlyMode{},
	)
	describeRoot, flags := newReleaseSetupWorkflowMissingRepository(t)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, describeRoot, "github-workflow-init", flags, releaseReadonlyMode{describe: true},
	)
	verboseRoot, flags := newReleaseSetupWorkflowMissingRepository(t)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, verboseRoot, "github-workflow-init", flags, releaseReadonlyMode{verbose: true},
	)
	combinedRoot, flags := newReleaseSetupWorkflowMissingRepository(t)
	combinedOutput, combinedErr := executeReleaseReadonlyCommand(
		t, manifest, combinedRoot, "github-workflow-init", flags, releaseReadonlyMode{describe: true, verbose: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil || combinedErr != nil {
		t.Fatalf("workflow preview exits: default=%v describe=%v verbose=%v combined=%v", defaultErr, describeErr, verboseErr, combinedErr)
	}
	for _, want := range []string{
		"GitHub Actions workflow scaffolding preview", "Target: .github/workflows/release-api.yml",
		"Status: create", "Action: would-create", "Generated workflow",
	} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("workflow preview default omitted %q:\n%s", want, defaultOutput)
		}
	}
	for _, hidden := range []string{
		"Workflow Identity", "Target Comparison", "Validation Facts", "Required Workflow Inputs", "Write Plan", "Limitations",
	} {
		if strings.Contains(defaultOutput, hidden) ||
			!strings.Contains(describeOutput, hidden) || !strings.Contains(combinedOutput, hidden) {
			t.Fatalf("workflow describe visibility for %q is incorrect:\ndefault:\n%s\ndescribe:\n%s\ncombined:\n%s", hidden, defaultOutput, describeOutput, combinedOutput)
		}
	}
	for _, want := range []string{
		"Validating workflow initialization request", "Reading Release V2 workflow configuration",
		"Resolving configured workflow target", "Reading existing workflow target",
		"Comparing canonical and existing workflow content", "Workflow target classified as create",
		"Dry-run selected; no workflow file written", "Workflow preview completed",
	} {
		if !strings.Contains(verboseOutput, want) || !strings.Contains(combinedOutput, want) {
			t.Fatalf("workflow verbose modes omitted %q:\nverbose:\n%s\ncombined:\n%s", want, verboseOutput, combinedOutput)
		}
	}
	outputs := []struct {
		root   string
		output string
	}{
		{root: defaultRoot, output: defaultOutput},
		{root: describeRoot, output: describeOutput},
		{root: verboseRoot, output: verboseOutput},
		{root: combinedRoot, output: combinedOutput},
	}
	for _, result := range outputs {
		for _, forbidden := range []string{result.root, "\x1b[", "setup-transport-secret", "Authorization", "Bearer"} {
			if strings.Contains(result.output, forbidden) {
				t.Fatalf("workflow human output contains %q:\n%s", forbidden, result.output)
			}
		}
		if existsReleaseSetupFile(filepath.Join(result.root, ".github", "workflows", "release-api.yml")) {
			t.Fatalf("workflow dry-run created target in %s", result.root)
		}
	}

	actualRoot, actualFlags := newReleaseSetupWorkflowMissingRepository(t)
	target := filepath.Join(actualRoot, ".github", "workflows", "release-api.yml")
	actualOutput, actualErr := executeReleaseReadonlyCommand(
		t, manifest, actualRoot, "github-workflow-init", actualFlags[:len(actualFlags)-1], releaseReadonlyMode{verbose: true},
	)
	if actualErr != nil {
		t.Fatalf("workflow actual create exit: %v\n%s", actualErr, actualOutput)
	}
	for _, want := range []string{
		"GitHub Workflow Initialization", "Workflow created", ".github/workflows/release-api.yml",
		"Writing missing workflow file", "Workflow file created", "Workflow initialization completed",
	} {
		if !strings.Contains(actualOutput, want) {
			t.Fatalf("workflow actual create omitted %q:\n%s", want, actualOutput)
		}
	}
	canonical, err := releasecmd.RenderCanonicalGitHubActionsReleaseWorkflow()
	if err != nil {
		t.Fatalf("render canonical workflow: %v", err)
	}
	created, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(created, canonical) {
		t.Fatalf("workflow actual create bytes differ from canonical: err=%v", err)
	}

	identicalOutput, identicalErr := executeReleaseReadonlyCommand(
		t, manifest, actualRoot, "github-workflow-init", actualFlags[:len(actualFlags)-1], releaseReadonlyMode{verbose: true},
	)
	if identicalErr != nil {
		t.Fatalf("workflow identical exit: %v\n%s", identicalErr, identicalOutput)
	}
	for _, want := range []string{"Workflow already current", "No write required", "Existing canonical workflow accepted; no write required"} {
		if !strings.Contains(identicalOutput, want) {
			t.Fatalf("workflow identical omitted %q:\n%s", want, identicalOutput)
		}
	}
	afterIdentical, err := os.ReadFile(target)
	if err != nil || !reflect.DeepEqual(afterIdentical, canonical) {
		t.Fatalf("workflow identical changed canonical target: err=%v", err)
	}

	conflictRoot, conflictFlags := newReleaseSetupWorkflowMissingRepository(t)
	conflictTarget := filepath.Join(conflictRoot, ".github", "workflows", "release-api.yml")
	conflicting := []byte("name: consumer-owned\n")
	writeReleaseSetupFile(t, conflictTarget, string(conflicting))
	conflictOutput, conflictErr := executeReleaseReadonlyCommand(
		t, manifest, conflictRoot, "github-workflow-init", conflictFlags[:len(conflictFlags)-1], releaseReadonlyMode{},
	)
	if conflictErr == nil {
		t.Fatal("workflow conflict changed non-zero exit policy")
	}
	for _, want := range []string{
		"Workflow Initialization Blocked", "WORKFLOW_TARGET_CONFLICT", "Different content",
		"Overwrite", "Refused", "--dry-run", "resolve the file manually",
	} {
		if !strings.Contains(conflictOutput, want) {
			t.Fatalf("workflow conflict omitted %q:\n%s", want, conflictOutput)
		}
	}
	afterConflict, err := os.ReadFile(conflictTarget)
	if err != nil || !reflect.DeepEqual(afterConflict, conflicting) {
		t.Fatalf("workflow conflict overwrote target: content=%q err=%v", afterConflict, err)
	}
}

func TestReleaseSetupCommandHelpDeclaresSupportedCoreFormats(t *testing.T) {
	manifestData, err := os.ReadFile(filepath.Join("..", "plugin", "release", "manifest.json"))
	if err != nil {
		t.Fatalf("read release manifest: %v", err)
	}
	var manifest plugin.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode release manifest: %v", err)
	}
	for _, command := range []string{"init", "unit-add", "migrate", "github-workflow-init"} {
		t.Run(command, func(t *testing.T) {
			root := testRootWithPluginResponseFlags()
			root.AddCommand(CreatePluginCommand(manifest))
			output, executeErr := executeTestCommand(root, "release", command, "--help")
			if executeErr != nil {
				t.Fatalf("%s help: %v", command, executeErr)
			}
			if !strings.Contains(output, "Outputs: table, json") || strings.Contains(output, "Outputs: text") {
				t.Fatalf("%s help declares unsupported outputs:\n%s", command, output)
			}
			commandStart := strings.Index(output, "\nCommand flags:\n")
			globalStart := strings.Index(output, "\nGlobal plugin-response flags:\n")
			usageStart := strings.Index(output, "\nUsage:")
			if commandStart < 0 || globalStart <= commandStart || usageStart <= globalStart {
				t.Fatalf("%s help sections are malformed:\n%s", command, output)
			}
			commandFlags := output[commandStart:globalStart]
			globalFlags := output[globalStart:usageStart]
			for _, inherited := range []string{"--describe", "--verbose", "--output"} {
				if strings.Contains(commandFlags, inherited) || !strings.Contains(globalFlags, inherited) {
					t.Fatalf("%s help does not keep %s inherited:\n%s", command, inherited, output)
				}
			}
		})
	}
}

func TestReleaseSetupCombinedOutputIsSafeWhenRedirectedWithNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	manifest := installReleaseSetupHelperPlugin(t)
	tests := []struct {
		fixture func(*testing.T) (string, []string)
		name    string
		command string
	}{
		{name: "init", command: "init", fixture: newReleaseSetupInitRepository},
		{name: "unit add", command: "unit-add", fixture: newReleaseSetupUnitAddRepository},
		{name: "migrate", command: "migrate", fixture: newReleaseSetupMigrationRepository},
		{name: "workflow init", command: "github-workflow-init", fixture: newReleaseSetupWorkflowMissingRepository},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root, flags := test.fixture(t)
			output, err := executeReleaseReadonlyCommand(
				t, manifest, root, test.command, flags,
				releaseReadonlyMode{describe: true, verbose: true},
			)
			if err != nil {
				t.Fatalf("%s combined redirected output: %v\n%s", test.command, err, output)
			}
			if strings.TrimSpace(output) == "" {
				t.Fatalf("%s combined redirected output is empty", test.command)
			}
			for _, forbidden := range []string{
				root, "\x1b[", "setup-transport-secret", "GITHUB_TOKEN", "GH_TOKEN", "Authorization", "Bearer",
			} {
				if strings.Contains(output, forbidden) {
					t.Fatalf("%s redirected NO_COLOR output contains %q:\n%s", test.command, forbidden, output)
				}
			}
		})
	}
}

func normalizeReleaseSetupData(t *testing.T, data map[string]any, root string) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("encode setup data: %v", err)
	}
	normalized := strings.ReplaceAll(string(encoded), root, "<fixture-root>")
	var result map[string]any
	if err := json.Unmarshal([]byte(normalized), &result); err != nil {
		t.Fatalf("decode normalized setup data: %v", err)
	}
	return result
}

func installReleaseSetupHelperPlugin(t *testing.T) plugin.Manifest {
	t.Helper()
	manifestData, err := os.ReadFile(filepath.Join("..", "plugin", "release", "manifest.json"))
	if err != nil {
		t.Fatalf("read release manifest: %v", err)
	}
	var manifest plugin.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode release manifest: %v", err)
	}
	pluginDir := installFakePlugin(t, manifest)
	restorePluginDir(t, pluginDir)

	binaryPath := filepath.Join(pluginDir, manifest.Name, "plugin-"+manifest.Name)
	script := "#!/bin/sh\nexec \"$NEKO_RELEASE_SETUP_TEST_BINARY\" -test.run=^TestReleaseSetupPluginHelperProcess$\n"
	if err := os.WriteFile(binaryPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write setup helper plugin: %v", err)
	}
	t.Setenv(releaseSetupHelperEnvironment, "1")
	t.Setenv("NEKO_RELEASE_SETUP_TEST_BINARY", os.Args[0])
	t.Setenv("GITHUB_TOKEN", "setup-transport-secret")
	t.Setenv("GH_TOKEN", "setup-transport-secret")
	return manifest
}

func newReleaseSetupInitRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := newReleaseSetupRepository(t)
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-api.yml"), "name: release api\n")
	return root, []string{
		"--unit", "api",
		"--display-name", "API",
		"--version", "1.2.3",
		"--executor", "goreleaser",
		"--delivery", "github-actions",
		"--workflow", ".github/workflows/release-api.yml",
		"--tag-prefix", "api/v",
		"--working-directory", ".",
		"--paths", "services/api/**",
	}
}

func newReleaseSetupUnitAddRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := newReleaseSetupRepository(t)
	writeReleaseSetupFile(t, filepath.Join(root, ".neko", "release.config.json"), `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "api",
      "displayName": "API",
      "paths": ["services/api/**"],
      "workingDirectory": ".",
      "tagPrefix": "api/v",
      "executor": {
        "type": "goreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-api.yml"
      }
    }
  ]
}
`)
	writeReleaseSetupFile(t, filepath.Join(root, ".neko", "release.state.json"), `{
  "schemaVersion": 2,
  "units": {
    "api": {"version": "1.2.3"}
  }
}
`)
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-api.yml"), "name: release api\n")
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-web.yml"), "name: release web\n")
	return root, []string{
		"--unit", "web",
		"--display-name", "Web",
		"--version", "2.3.4",
		"--executor", "goreleaser",
		"--delivery", "github-actions",
		"--workflow", ".github/workflows/release-web.yml",
		"--tag-prefix", "web/v",
		"--working-directory", ".",
		"--paths", "services/web/**",
	}
}

func newReleaseSetupMigrationRepository(t *testing.T) (string, []string) {
	t.Helper()
	root := t.TempDir()
	runReleaseReadonlyGit(t, root, "init")
	writeReleaseSetupFile(t, filepath.Join(root, ".github", "workflows", "release-default.yml"), "name: release default\n")
	writeReleaseSetupFile(t, filepath.Join(root, ".release.neko.json"), `{
  "project-name": "example",
  "project-owner": "example-owner",
  "project-type": "backend",
  "release-system": "jreleaser",
  "version": "1.2.3"
}
`)
	return root, []string{"--dry-run"}
}

func newReleaseSetupMigrationActualRepository(t *testing.T) (string, []string) {
	t.Helper()
	root, _ := newReleaseSetupMigrationRepository(t)
	return root, nil
}

func newReleaseSetupWorkflowRepository(t *testing.T) (string, []string) {
	t.Helper()
	root, _ := newReleaseSetupUnitAddRepository(t)
	return root, []string{"--unit", "api", "--dry-run"}
}

func newReleaseSetupWorkflowMissingRepository(t *testing.T) (string, []string) {
	t.Helper()
	root, flags := newReleaseSetupWorkflowRepository(t)
	target := filepath.Join(root, ".github", "workflows", "release-api.yml")
	if err := os.Remove(target); err != nil {
		t.Fatalf("remove workflow fixture target: %v", err)
	}
	return root, flags
}

func newReleaseSetupWorkflowActualRepository(t *testing.T) (string, []string) {
	t.Helper()
	root, flags := newReleaseSetupWorkflowMissingRepository(t)
	return root, flags[:len(flags)-1]
}

func newReleaseSetupRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create Git marker: %v", err)
	}
	return root
}

func writeReleaseSetupFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func existsReleaseSetupFile(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func snapshotReleaseSetupFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot setup fixture: %v", err)
	}
	return result
}

const releaseSetupMigratedConfig = `{
  "schemaVersion": 2,
  "units": [
    {
      "id": "default",
      "displayName": "example",
      "paths": ["**"],
      "workingDirectory": ".",
      "tagPrefix": "v",
      "executor": {
        "type": "jreleaser",
        "delivery": "github-actions",
        "workflow": ".github/workflows/release-default.yml"
      }
    }
  ]
}
`

const releaseSetupMigratedState = `{
  "schemaVersion": 2,
  "units": {
    "default": {"version": "1.2.3"}
  }
}
`
