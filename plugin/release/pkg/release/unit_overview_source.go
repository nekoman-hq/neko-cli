package release

import (
	"context"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasesource"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type unitOverviewSourceReader interface {
	Read(string) releasesource.Snapshot
}

type unitOverviewInspector interface {
	Inspect(context.Context, unitOverviewRequest) *unitOverviewResult
}

type unitOverviewInspectionUseCase struct {
	sources unitOverviewSourceReader
}

func (useCase unitOverviewInspectionUseCase) Inspect(
	_ context.Context,
	request unitOverviewRequest,
) *unitOverviewResult {
	result := &unitOverviewResult{
		Units:         make([]unitOverviewRow, 0),
		WorkflowPaths: make([]string, 0),
	}
	snapshot := useCase.sources.Read(request.RepositoryRoot)
	if sourceIssue := classifyUnitOverviewSource(snapshot); sourceIssue != nil {
		result.SourceIssue = sourceIssue
	}
	if !unitOverviewSourceCanProduceRows(snapshot) {
		finalizeUnitOverviewResult(result)
		return result
	}

	if snapshot.Config != nil && snapshot.State != nil {
		if err := releaseconfig.ValidateV2ConfigStateStructure(snapshot.Config, snapshot.State); err == nil {
			repository := releaseconfig.NormalizeV2Repository(request.RepositoryRoot, snapshot.Config, snapshot.State)
			result.Units = deriveCanonicalUnitOverviewRows(repository)
			result.SourceUsable = true
			finalizeUnitOverviewResult(result)
			return result
		}
	}

	result.Units = derivePartialUnitOverviewRows(snapshot.Config, snapshot.State)
	result.SourceUsable = snapshot.Config != nil && snapshot.State != nil && result.SourceIssue == nil
	if len(result.Units) == 0 && result.SourceIssue == nil {
		result.SourceIssue = newUnitOverviewSourceIssue(
			"V2_SOURCE_EMPTY",
			"Release V2 config and state do not contain any inspectable units.",
			"Define at least one canonical Release V2 unit in config and state.",
		)
	}
	finalizeUnitOverviewResult(result)
	return result
}

func classifyUnitOverviewSource(snapshot releasesource.Snapshot) *unitOverviewIssue {
	switch {
	case snapshot.InspectionErr != nil:
		return newUnitOverviewSourceIssue(
			"V2_SOURCE_INSPECTION_FAILED",
			"Release source files could not be inspected safely.",
			"Restore local read access to the repository root and source files.",
		)
	case snapshot.V1Present && (snapshot.ConfigPresent || snapshot.StatePresent):
		return newUnitOverviewSourceIssue(
			"MIXED_RELEASE_SOURCES",
			"V1 and V2 release sources coexist at the repository root.",
			"Complete and verify V2 migration, then remove the obsolete V1 source.",
		)
	case snapshot.V1Present:
		return newUnitOverviewSourceIssue(
			"V1_SOURCE_UNSUPPORTED",
			"The unit overview supports Release V2 repositories only.",
			"Migrate the repository to the V2 config/state pair.",
		)
	case !snapshot.ConfigPresent && !snapshot.StatePresent:
		return newUnitOverviewSourceIssue(
			"V2_SOURCE_MISSING",
			"Release V2 config and state are missing.",
			"Create the canonical Release V2 config/state pair.",
		)
	case snapshot.ConfigError != nil:
		return newUnitOverviewSourceIssue(
			"V2_CONFIG_INVALID",
			".neko/release.config.json is malformed, unreadable, or not strict JSON.",
			"Repair the V2 configuration using the canonical schema.",
		)
	case snapshot.StateError != nil:
		return newUnitOverviewSourceIssue(
			"V2_STATE_INVALID",
			".neko/release.state.json is malformed, unreadable, or not strict JSON.",
			"Repair the V2 state using the canonical schema.",
		)
	case snapshot.Config != nil && snapshot.Config.SchemaVersion != 2:
		return newUnitOverviewSourceIssue(
			"V2_SCHEMA_UNSUPPORTED",
			"The Release V2 config schemaVersion is not exactly 2.",
			"Migrate the configuration to schemaVersion 2.",
		)
	case snapshot.State != nil && snapshot.State.SchemaVersion != 2:
		return newUnitOverviewSourceIssue(
			"V2_SCHEMA_UNSUPPORTED",
			"The Release V2 state schemaVersion is not exactly 2.",
			"Migrate the state to schemaVersion 2.",
		)
	case snapshot.ConfigPresent && snapshot.StatePresent && !snapshot.RecoveryReady:
		return newUnitOverviewSourceIssue(
			"V2_RECOVERY_BLOCKED",
			"Unresolved V2 pair recovery evidence makes config/state facts uncertain.",
			"Inspect and resolve the local V2 recovery evidence before relying on the overview.",
		)
	case !snapshot.ConfigPresent:
		return newUnitOverviewSourceIssue(
			"V2_CONFIG_MISSING",
			".neko/release.config.json is missing.",
			"Restore the canonical Release V2 configuration.",
		)
	case !snapshot.StatePresent:
		return newUnitOverviewSourceIssue(
			"V2_STATE_MISSING",
			".neko/release.state.json is missing.",
			"Restore the canonical Release V2 state file.",
		)
	default:
		return nil
	}
}

func unitOverviewSourceCanProduceRows(snapshot releasesource.Snapshot) bool {
	if snapshot.InspectionErr != nil || snapshot.V1Present || snapshot.ConfigError != nil || snapshot.StateError != nil {
		return false
	}
	if snapshot.Config != nil && snapshot.Config.SchemaVersion != 2 {
		return false
	}
	if snapshot.State != nil && snapshot.State.SchemaVersion != 2 {
		return false
	}
	if snapshot.ConfigPresent && snapshot.StatePresent && !snapshot.RecoveryReady {
		return false
	}
	return snapshot.Config != nil || snapshot.State != nil
}

func newUnitOverviewSourceIssue(code, message, remediation string) *unitOverviewIssue {
	return &unitOverviewIssue{
		Severity: unitOverviewIssueError,
		Code:     code, Message: message, Remediation: remediation,
	}
}

var _ unitOverviewSourceReader = releasesource.FilesystemReader{}
var _ unitOverviewInspector = unitOverviewInspectionUseCase{}
