package release

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type LocalPlanReadiness string

const (
	LocalPlanReady       LocalPlanReadiness = "ready"
	LocalPlanBlocked     LocalPlanReadiness = "blocked"
	LocalPlanUnsupported LocalPlanReadiness = "unsupported"
)

const legacyReleaseConfigFileName = ".release.neko.json"

type ReleasePlanInspectionRequest struct {
	ReleaseType Type
	UnitID      string
}

//nolint:govet // Inspection result fields follow output order.
type ReleasePlanInspection struct {
	Source              releaseconfig.SourceFormat
	Unit                ReleasePlanInspectionUnit
	CurrentVersion      string
	RequestedChange     Type
	NextVersion         string
	Tag                 string
	Executor            string
	Delivery            string
	Workflow            string
	WorkingDirectory    string
	UnitRoot            string
	MaterializedOutputs []PlannedMaterializedOutput
	KnownReleaseFiles   []InspectedKnownReleaseFile
	Readiness           LocalPlanReadiness
	Blockers            []LocalPlanBlocker
	Limitations         []ReleasePlanLimitation
}

type ReleasePlanInspectionUnit struct {
	ID          string
	DisplayName string
}

type PlannedMaterializedOutput struct {
	Path                     string
	Reason                   string
	RequiredForReleaseCommit bool
	Exists                   bool
}

type InspectedKnownReleaseFile struct {
	Path                     string
	Reason                   string
	RequiredForReleaseCommit bool
}

type LocalPlanBlocker struct {
	Category string
	Message  string
}

type ReleasePlanLimitation struct {
	Category string
	Message  string
}

type releasePlanInspectionUseCase struct {
	repositories   releaseRepositoryReader
	repositoryRoot string
}

func newReleasePlanInspectionUseCase(repositoryRoot string) releasePlanInspectionUseCase {
	return releasePlanInspectionUseCase{
		repositoryRoot: repositoryRoot,
		repositories:   releaseConfigRepositoryReader{},
	}
}

func (useCase releasePlanInspectionUseCase) Inspect(_ context.Context, request ReleasePlanInspectionRequest) (*ReleasePlanInspection, *CommandFailure) {
	repository, err := useCase.repositories.Load(useCase.repositoryRoot)
	if err != nil {
		return nil, &CommandFailure{
			Code:  "CONFIG_NOT_FOUND",
			Cause: err,
			Details: map[string]any{
				"hint": "Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config",
			},
		}
	}
	path, err := selectReleaseApplicationPath(repository.SourceFormat)
	if err != nil {
		return nil, failureFromError("SOURCE_FORMAT_UNSUPPORTED", err)
	}

	switch path {
	case releaseconfig.SourceFormatV1:
		return inspectV1ReleasePlan(repository, request)
	case releaseconfig.SourceFormatV2:
		return inspectV2ReleasePlan(repository, request)
	default:
		return nil, failureFromMessage("SOURCE_FORMAT_UNSUPPORTED", "release source selection returned no application path")
	}
}

func inspectV1ReleasePlan(repository *releaseconfig.ReleaseRepository, request ReleasePlanInspectionRequest) (*ReleasePlanInspection, *CommandFailure) {
	unit, err := releaseconfig.ResolveReleaseUnit(repository, request.UnitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	if repository.Legacy == nil {
		return nil, failureFromMessage("CONFIG_NOT_FOUND", "v1 release configuration is missing")
	}
	intent := V1ReleaseIntent{
		RepositoryRoot: repository.RepositoryRoot,
		Unit:           *unit,
		Config:         repository.Legacy,
		ReleaseType:    request.ReleaseType,
	}
	plan, err := PlanV1Release(V1ReleasePlanningRequest{Intent: intent})
	if err != nil {
		return nil, failureFromError("VERSION_ERROR", err)
	}
	return &ReleasePlanInspection{
		Source:           releaseconfig.SourceFormatV1,
		Unit:             inspectionUnit(*unit),
		CurrentVersion:   plan.CurrentVersion,
		RequestedChange:  request.ReleaseType,
		NextVersion:      plan.NextVersion,
		Tag:              plan.Tag,
		Executor:         plan.Executor,
		Delivery:         string(releaseconfig.DeliveryLocal),
		WorkingDirectory: unit.WorkingDirectory,
		UnitRoot:         repository.RepositoryRoot,
		MaterializedOutputs: []PlannedMaterializedOutput{
			{
				Path:                     legacyReleaseConfigFileName,
				Reason:                   "sync V1 release config version with release plan",
				RequiredForReleaseCommit: true,
				Exists:                   true,
			},
		},
		Readiness: LocalPlanReady,
		Limitations: appendCommonPlanLimitations([]ReleasePlanLimitation{
			{
				Category: "v1-latest-tag-evidence",
				Message:  "V1 latest-tag evidence is not inspected by this command; existing V1 release execution keeps its compatibility behavior.",
			},
			{
				Category: "v1-known-release-files",
				Message:  "V1 releases do not use the V2 known-release-file allowlist; the legacy release config is the planned local version file.",
			},
		}),
	}, nil
}

func inspectV2ReleasePlan(repository *releaseconfig.ReleaseRepository, request ReleasePlanInspectionRequest) (*ReleasePlanInspection, *CommandFailure) {
	unit, err := releaseconfig.ResolveReleaseUnit(repository, request.UnitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	execCtx, err := BuildV2ReleaseExecutionContext(repository.RepositoryRoot, *unit, request.ReleaseType, true)
	if err != nil {
		return nil, failureFromError("EXECUTION_CONTEXT_FAILED", err)
	}
	facts, failure := planV2ReleaseFacts(execCtx)
	if failure != nil {
		return nil, failure
	}
	blockers := localPlanBlockersFromMaterialization(facts.MaterializationPlan)
	readiness := LocalPlanReady
	if len(blockers) > 0 {
		readiness = LocalPlanBlocked
	}
	return &ReleasePlanInspection{
		Source:              releaseconfig.SourceFormatV2,
		Unit:                inspectionUnit(*unit),
		CurrentVersion:      execCtx.CurrentVersion,
		RequestedChange:     request.ReleaseType,
		NextVersion:         execCtx.NextVersion,
		Tag:                 execCtx.Tag,
		Executor:            execCtx.Executor,
		Delivery:            execCtx.Delivery,
		Workflow:            execCtx.Workflow,
		WorkingDirectory:    unit.WorkingDirectory,
		UnitRoot:            execCtx.UnitRoot,
		MaterializedOutputs: plannedMaterializedOutputs(facts.MaterializationPlan),
		KnownReleaseFiles:   inspectedKnownReleaseFiles(facts.KnownFiles),
		Readiness:           readiness,
		Blockers:            blockers,
		Limitations:         appendCommonPlanLimitations(nil),
	}, nil
}

func inspectionUnit(unit releaseconfig.ReleaseUnit) ReleasePlanInspectionUnit {
	return ReleasePlanInspectionUnit{
		ID:          unit.ID,
		DisplayName: strings.TrimSpace(unit.DisplayName),
	}
}

func plannedMaterializedOutputs(plan *MaterializationPlan) []PlannedMaterializedOutput {
	if plan == nil || len(plan.Changes) == 0 {
		return nil
	}
	outputs := make([]PlannedMaterializedOutput, 0, len(plan.Changes))
	for _, change := range plan.Changes {
		outputs = append(outputs, PlannedMaterializedOutput{
			Path:                     filepath.ToSlash(change.RepositoryRelativePath),
			Reason:                   change.Reason,
			RequiredForReleaseCommit: change.RequiredForReleaseCommit,
			Exists:                   change.Existed,
		})
	}
	sort.Slice(outputs, func(i, j int) bool { return outputs[i].Path < outputs[j].Path })
	return outputs
}

func inspectedKnownReleaseFiles(files KnownReleaseFiles) []InspectedKnownReleaseFile {
	inspected := make([]InspectedKnownReleaseFile, 0, len(files.Files))
	for _, file := range files.Files {
		inspected = append(inspected, InspectedKnownReleaseFile{
			Path:                     filepath.ToSlash(file.RepositoryRelativePath),
			Reason:                   file.Reason,
			RequiredForReleaseCommit: file.RequiredForReleaseCommit,
		})
	}
	sort.Slice(inspected, func(i, j int) bool { return inspected[i].Path < inspected[j].Path })
	return inspected
}

func localPlanBlockersFromMaterialization(plan *MaterializationPlan) []LocalPlanBlocker {
	if plan == nil || strings.TrimSpace(plan.BlockedReason) == "" {
		return nil
	}
	return []LocalPlanBlocker{
		{
			Category: "materialization-blocked",
			Message:  plan.BlockedReason,
		},
	}
}

func appendCommonPlanLimitations(limitations []ReleasePlanLimitation) []ReleasePlanLimitation {
	common := []ReleasePlanLimitation{
		{
			Category: "local-only",
			Message:  "This inspection uses local planning facts only and does not start release execution.",
		},
		{
			Category: "no-remote-checks",
			Message:  "Remote tags, releases, workflow runs, push permissions, and provider acceptance are not inspected.",
		},
		{
			Category: "token-free",
			Message:  "Tokens and provider authorization are not read or reported.",
		},
		{
			Category: "no-evidence-inspection",
			Message:  "Execution journals, dispatch journals, recovery evidence, and retry state are not inspected.",
		},
	}
	result := append([]ReleasePlanLimitation(nil), limitations...)
	result = append(result, common...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Category == result[j].Category {
			return result[i].Message < result[j].Message
		}
		return result[i].Category < result[j].Category
	})
	return result
}
