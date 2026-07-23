//nolint:staticcheck // Presentation contracts intentionally exercise deprecated V1 inputs.
package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestMigrationPresentationSeparatesDefaultAndDescribeAndPreservesJSON(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)

	response, err := HandleMigrate(plugin.Request{
		Flags:   map[string]any{"dry-run": true},
		Context: plugin.Context{WorkingDir: root},
	})
	if err != nil || response.Status != "success" {
		t.Fatalf("dry-run response=%#v err=%v", response, err)
	}

	plain := renderMigrationResponse(t, response, false)
	described := renderMigrationResponse(t, response, true)
	for _, want := range []string{
		"Release Migration", "Dry run", "Yes", "Source contract", "V1", "Destination contract", "V2",
		"Planned actions", "7", "Configuration", ".neko/release.config.json",
		"State", ".neko/release.state.json", "Archive", ".release.neko.json.v1.bak",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("migration default omitted %q:\n%s", want, plain)
		}
	}
	for _, hidden := range []string{
		"Source Facts", "Resolved V2 Configuration", "Generated Artifacts",
		"Ordered Migration Plan", "Archive and Journal", "Validation Facts", "Limitations",
	} {
		if strings.Contains(plain, hidden) || !strings.Contains(described, hidden) {
			t.Fatalf("migration describe visibility for %q is incorrect:\nplain:\n%s\ndescribed:\n%s", hidden, plain, described)
		}
	}
	if strings.Contains(plain, `"schemaVersion"`) {
		t.Fatalf("migration default dumped generated JSON:\n%s", plain)
	}
	if strings.Contains(plain, root) || strings.Contains(described, root) {
		t.Fatalf("migration human output exposed absolute fixture root:\nplain:\n%s\ndescribed:\n%s", plain, described)
	}
	assertMigrationJSONPresentationInvariant(t, response)
}

func TestMigrationFailurePresentationIsActionableAndSafe(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), "{")

	response, err := HandleMigrate(plugin.Request{Context: plugin.Context{WorkingDir: root}})
	if err != nil || response.Status != "error" {
		t.Fatalf("failure response=%#v err=%v", response, err)
	}
	output := renderMigrationResponse(t, response, false)
	for _, want := range []string{"Migration Blocked", "V1 source", "Refused", "No files written", "Next action"} {
		if !strings.Contains(output, want) {
			t.Fatalf("migration failure omitted %q:\n%s", want, output)
		}
	}
	for _, forbidden := range []string{root, "\x1b[", "Authorization", "Bearer"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("migration failure output contains %q:\n%s", forbidden, output)
		}
	}
}

func TestMigrationAlreadyCompletedPresentationAndExecutionFailureEvidenceAreAccurate(t *testing.T) {
	root := withGitRepo(t)
	writeMigratedV2(t, root)
	before := snapshotMigrationPresentationFiles(t, root)
	response, err := HandleMigrate(plugin.Request{Context: plugin.Context{WorkingDir: root}})
	if err != nil || response.Status != "success" {
		t.Fatalf("already-migrated response=%#v err=%v", response, err)
	}
	plain := renderMigrationResponse(t, response, false)
	described := renderMigrationResponse(t, response, true)
	for _, want := range []string{"Already migrated; no changes required", "Archive", "Not required", "No migration action required"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("already-migrated default omitted %q:\n%s", want, plain)
		}
	}
	for _, want := range []string{"Existing V2 pair validated", "No change required", "existing V2 configuration"} {
		if !strings.Contains(described, want) {
			t.Fatalf("already-migrated describe omitted %q:\n%s", want, described)
		}
	}
	after := snapshotMigrationPresentationFiles(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("already-migrated command changed fixture files")
	}

	failure := &migrationFailure{
		kind: migrationTargetVerificationFailure, cause: os.ErrInvalid,
	}
	failureOutput := renderMigrationResponse(t, mapMigrationCommandResponse(
		migrationCommandResult{}, failure, time.Time{},
	), false)
	for _, want := range []string{
		"V2 target verification", "Stopped; recovery evidence may remain",
		"Do not archive V1", "MIGRATION_FAILED",
	} {
		if !strings.Contains(failureOutput, want) {
			t.Fatalf("execution failure presentation omitted %q:\n%s", want, failureOutput)
		}
	}
	if strings.Contains(failureOutput, "No files written") {
		t.Fatalf("execution failure incorrectly claimed no writes:\n%s", failureOutput)
	}
}

func TestMigrationPresentationRemainsReadableAtNarrowAndUnknownWidth(t *testing.T) {
	root := withGitRepo(t)
	writeFile(t, filepath.Join(root, releaseconfig.V1FileName), v1Fixture)
	response, err := HandleMigrate(plugin.Request{
		Flags: map[string]any{"dry-run": true}, Context: plugin.Context{WorkingDir: root},
	})
	if err != nil || response.Status != "success" {
		t.Fatalf("dry-run response=%#v err=%v", response, err)
	}
	for _, width := range []migrationPresentationWidthState{{width: 34, available: true}, {available: false}} {
		var output bytes.Buffer
		if renderErr := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
			Format: renderer.FormatTable, Describe: true, WidthProvider: width,
		}, &output); renderErr != nil {
			t.Fatalf("render migration width %#v: %v", width, renderErr)
		}
		for _, want := range []string{"Release Migration", "Ordered Migration Plan", ".neko/release.config.json"} {
			if !strings.Contains(output.String(), want) {
				t.Fatalf("migration width %#v omitted %q:\n%s", width, want, output.String())
			}
		}
		if strings.Contains(output.String(), root) || strings.Contains(output.String(), "\x1b[") {
			t.Fatalf("migration width %#v exposed unsafe output:\n%s", width, output.String())
		}
	}
}

func TestMigrationVerboseDistinguishesDryRunAndActualWrites(t *testing.T) {
	dryRunRoot := withGitRepo(t)
	dryRunSource := filepath.Join(dryRunRoot, releaseconfig.V1FileName)
	writeFile(t, dryRunSource, v1Fixture)
	dryRunOutput := captureMigrationLogs(t, func() {
		response, err := HandleMigrate(plugin.Request{
			Flags:   map[string]any{"dry-run": true},
			Context: plugin.Context{WorkingDir: dryRunRoot},
		})
		if err != nil || response.Status != "success" {
			t.Fatalf("dry-run response=%#v err=%v", response, err)
		}
	})
	assertMigrationLogOrder(t, dryRunOutput, []string{
		"Resolving migration repository root",
		"Locating V1 release configuration",
		"Validating V1 migration source",
		"Deriving V2 release configuration",
		"Deriving V2 release state",
		"Validating generated V2 migration artifacts",
		"Planning archive and migration journal actions",
		"Evaluating migration dry-run decision",
		"Dry-run selected; no migration files written",
		"Migration planning completed",
	})
	for _, forbidden := range []string{
		"V2 configuration and state written", "Legacy V1 configuration archived",
		dryRunRoot, "\x1b[", "GITHUB_TOKEN",
	} {
		if strings.Contains(dryRunOutput, forbidden) {
			t.Fatalf("dry-run log contains %q:\n%s", forbidden, dryRunOutput)
		}
	}
	if !exists(dryRunSource) || exists(releaseconfig.V2ConfigPath(dryRunRoot)) ||
		exists(releaseconfig.V2StatePath(dryRunRoot)) || exists(filepath.Join(dryRunRoot, backupFileName)) ||
		exists(filepath.Join(dryRunRoot, releaseconfig.V2Directory, journalFileName)) {
		t.Fatal("dry-run verbose path changed migration files")
	}

	actualRoot := withGitRepo(t)
	actualSource := filepath.Join(actualRoot, releaseconfig.V1FileName)
	writeFile(t, actualSource, v1Fixture)
	actualOutput := captureMigrationLogs(t, func() {
		response, err := HandleMigrate(plugin.Request{Context: plugin.Context{WorkingDir: actualRoot}})
		if err != nil || response.Status != "success" {
			t.Fatalf("actual response=%#v err=%v", response, err)
		}
	})
	assertMigrationLogOrder(t, actualOutput, []string{
		"Evaluating migration dry-run decision",
		"Preparing migration recovery journal",
		"Writing V2 configuration and state",
		"V2 configuration and state written",
		"Validating persisted V2 migration artifacts",
		"Archiving legacy V1 configuration",
		"Legacy V1 configuration archived",
		"Completing migration journal",
		"Migration execution completed",
		"Migration command completed",
	})
	for _, forbidden := range []string{actualRoot, "\x1b[", "GITHUB_TOKEN", "Authorization", "Bearer"} {
		if strings.Contains(actualOutput, forbidden) {
			t.Fatalf("actual migration log contains %q:\n%s", forbidden, actualOutput)
		}
	}
	if exists(actualSource) || !exists(releaseconfig.V2ConfigPath(actualRoot)) ||
		!exists(releaseconfig.V2StatePath(actualRoot)) || !exists(filepath.Join(actualRoot, backupFileName)) ||
		exists(filepath.Join(actualRoot, releaseconfig.V2Directory, journalFileName)) {
		t.Fatal("actual migration verbose path did not preserve exact write/archive behavior")
	}
}

func renderMigrationResponse(t *testing.T, response *plugin.Response, describe bool) string {
	t.Helper()
	var output bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatTable, Describe: describe, WidthProvider: migrationPresentationWidth(120),
	}, &output); err != nil {
		t.Fatalf("render migration response: %v", err)
	}
	return output.String()
}

func assertMigrationJSONPresentationInvariant(t *testing.T, response *plugin.Response) {
	t.Helper()
	var plain bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatJSON}, &plain); err != nil {
		t.Fatalf("render migration JSON: %v", err)
	}
	var described bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{
		Format: renderer.FormatJSON, Describe: true,
	}, &described); err != nil {
		t.Fatalf("render described migration JSON: %v", err)
	}
	if !reflect.DeepEqual(plain.Bytes(), described.Bytes()) {
		t.Fatalf("describe changed migration JSON:\nplain=%s\ndescribed=%s", plain.String(), described.String())
	}
	for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b["} {
		if strings.Contains(plain.String(), forbidden) {
			t.Fatalf("migration JSON contains %q:\n%s", forbidden, plain.String())
		}
	}
}

func captureMigrationLogs(t *testing.T, run func()) string {
	t.Helper()
	previousVerbose := log.Verbose
	log.Verbose = true
	t.Cleanup(func() { log.Verbose = previousVerbose })
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}
	previousStderr := os.Stderr
	os.Stderr = writer
	run()
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatalf("close stderr writer: %v", closeErr)
	}
	os.Stderr = previousStderr
	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	if closeErr := reader.Close(); closeErr != nil {
		t.Fatalf("close stderr reader: %v", closeErr)
	}
	return ansi.Strip(string(content))
}

func assertMigrationLogOrder(t *testing.T, output string, phases []string) {
	t.Helper()
	previous := -1
	for _, phase := range phases {
		index := strings.Index(output, phase)
		if index < 0 {
			t.Fatalf("migration verbose output omitted %q:\n%s", phase, output)
		}
		if index <= previous {
			t.Fatalf("migration verbose phase %q is out of order:\n%s", phase, output)
		}
		previous = index
	}
}

type migrationPresentationWidth int

func (width migrationPresentationWidth) Width(io.Writer) (int, bool) {
	return int(width), true
}

type migrationPresentationWidthState struct {
	width     int
	available bool
}

func (width migrationPresentationWidthState) Width(io.Writer) (int, bool) {
	return width.width, width.available
}

func snapshotMigrationPresentationFiles(t *testing.T, root string) map[string]string {
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
		t.Fatalf("snapshot migration fixture: %v", err)
	}
	return result
}
