package pipelineinspection

import (
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasesource"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func inspectConfiguredPipeline(request pipelineRequest, stages []LifecycleStage) (*pipelineResult, *commandFailure) {
	snapshot := (releasesource.FilesystemReader{}).Read(request.RepositoryRoot)
	if failure := classifyPipelineSource(snapshot); failure != nil {
		return nil, failure
	}
	repository, err := releaseconfig.LoadV2Repository(request.RepositoryRoot)
	if err != nil {
		return nil, classifyPipelineConfigurationError(err)
	}
	unit, err := releaseconfig.ResolveReleaseUnit(repository, request.UnitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, &commandFailure{Code: "PIPELINE_UNIT_INVALID", Message: err.Error()}
	}
	identity, err := releasetool.ParseIdentity(unit.ExecutorType)
	if err != nil {
		return nil, &commandFailure{Code: "PIPELINE_EXECUTOR_UNSUPPORTED", Message: "The selected unit uses an unsupported release executor."}
	}
	if unit.Delivery != string(releaseconfig.DeliveryGitHubActions) {
		return nil, &commandFailure{Code: "PIPELINE_DELIVERY_UNSUPPORTED", Message: "The selected unit must use github-actions delivery."}
	}
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return nil, &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "The selected unit has an invalid tag prefix."}
	}

	requiredInputs := make([]string, 0)
	for _, input := range releaseworkflow.CanonicalDispatchInputContract() {
		requiredInputs = append(requiredInputs, input.Name)
	}
	kind := unit.Kind
	if kind == "" {
		kind = "release"
	}
	return &pipelineResult{
		SchemaVersion: 1,
		Status:        pipelineReady,
		Unit: pipelineUnit{
			ID: unit.ID, DisplayName: unit.DisplayName, Kind: kind,
			Executor: string(identity), Delivery: unit.Delivery,
			ConfiguredVersion: unit.Version, WorkingDirectory: unit.WorkingDirectory,
		},
		Release: pipelineRelease{
			ConfiguredVersion: unit.Version, TagPrefix: unit.TagPrefix,
			ConfiguredTag:     tagSpec.Format(unit.Version),
			MaterializedFiles: make([]pipelineMaterializedFile, 0),
		},
		Repository: pipelineRepository{
			SourceGeneration: "v2", LocalBranch: "not_inspected",
			LocalHead: "not_inspected", Tracking: "not_inspected",
		},
		Workflow: pipelineWorkflow{
			Path: unit.Workflow, Delivery: unit.Delivery,
			RequiredInputs: requiredInputs, ReleaseTool: string(identity),
			ConsumerOperations: make([]string, 0), Publication: "configured_not_inspected",
			PluginRegistry: pluginRegistryStatus(unit.IsPlugin),
		},
		Stages: append(make([]LifecycleStage, 0, len(stages)), stages...),
		ProgressInspection: pipelineProgressInspection{
			ExecutionProgress: "not_inspected", JournalsInspected: false,
			ResumeEligibilityEvaluated: false, RemoteStateInspected: false,
		},
		Limitations: []string{
			"Execution journals and current release progress were not inspected.",
			"Resume eligibility was not evaluated.",
			"Remote workflow and publication state were not inspected.",
			"Runtime execution success cannot be guaranteed from local configuration.",
		},
	}, nil
}

func classifyPipelineSource(snapshot releasesource.Snapshot) *commandFailure {
	switch {
	case snapshot.InspectionErr != nil:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Release source files could not be inspected safely."}
	case snapshot.V1Present && (snapshot.ConfigPresent || snapshot.StatePresent):
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "V1 and V2 release sources coexist at the repository root."}
	case snapshot.V1Present:
		return &commandFailure{Code: "PIPELINE_SOURCE_UNSUPPORTED", Message: "Pipeline inspection supports Release V2 sources only; migrate the repository to the V2 config/state pair."}
	case !snapshot.ConfigPresent && !snapshot.StatePresent:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Release V2 config and state are missing."}
	case !snapshot.ConfigPresent:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Release V2 config is missing."}
	case !snapshot.StatePresent:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Release V2 state is missing."}
	case snapshot.ConfigError != nil:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Release V2 config is invalid."}
	case snapshot.StateError != nil:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Release V2 state is invalid."}
	case !snapshot.RecoveryReady:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: "Unresolved V2 pair recovery evidence makes the release source uncertain."}
	default:
		return nil
	}
}

func classifyPipelineConfigurationError(err error) *commandFailure {
	message := err.Error()
	switch {
	case strings.Contains(message, "executor"):
		return &commandFailure{Code: "PIPELINE_EXECUTOR_UNSUPPORTED", Message: "Release V2 configuration contains an unsupported executor."}
	case strings.Contains(message, "delivery"):
		return &commandFailure{Code: "PIPELINE_DELIVERY_UNSUPPORTED", Message: "Release V2 configuration contains an unsupported delivery."}
	default:
		return &commandFailure{Code: "PIPELINE_SOURCE_INVALID", Message: fmt.Sprintf("Release V2 configuration cannot be inspected: %s", sanitizeConfigurationError(message))}
	}
}

func sanitizeConfigurationError(message string) string {
	if strings.Contains(message, "/") || strings.Contains(message, "\\") {
		return "the configured repository contract is invalid"
	}
	return message
}

func pluginRegistryStatus(isPlugin bool) string {
	if isPlugin {
		return "configured_for_plugin_unit"
	}
	return "not_applicable"
}
