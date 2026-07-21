package pipelineinspection

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasesource"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func inspectConfiguredPipeline(request pipelineRequest, stages []LifecycleStage, runtime RuntimeSnapshot) (*pipelineResult, *commandFailure) {
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
	consumerFacts, failure := inspectPipelineConsumerWorkflow(request.RepositoryRoot, *unit)
	if failure != nil {
		return nil, failure
	}
	consumerStages := pipelineConsumerStages(unit.Workflow, consumerFacts)
	allStages := make([]LifecycleStage, 0, len(stages)+len(consumerStages))
	allStages = append(allStages, stages...)
	allStages = append(allStages, consumerStages...)
	consumerOperations := make([]string, 0, len(consumerFacts.Operations))
	for _, operation := range consumerFacts.Operations {
		consumerOperations = append(consumerOperations, string(operation.ID))
	}

	requiredInputs := make([]string, 0)
	for _, input := range releaseworkflow.CanonicalDispatchInputContract() {
		requiredInputs = append(requiredInputs, input.Name)
	}
	kind := unit.Kind
	if kind == "" {
		kind = "release"
	}
	result := &pipelineResult{
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
			MaterializedFiles: configuredMaterializedFiles(*unit, identity),
		},
		Repository: pipelineRepository{
			SourceGeneration: "v2", LocalBranch: "not_inspected",
			LocalHead: "not_inspected", Tracking: "not_inspected",
		},
		Workflow: pipelineWorkflow{
			Path: unit.Workflow, Delivery: unit.Delivery,
			RequiredInputs: requiredInputs, ReleaseTool: string(identity),
			ConsumerOperations: consumerOperations,
			Publication:        configuredPublicationStatus(consumerFacts),
			PluginRegistry:     configuredPluginRegistryStatus(unit.IsPlugin, consumerFacts),
		},
		Stages: allStages,
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
	}
	applyPipelineRuntime(result, runtime)
	return result, nil
}

func inspectPipelineConsumerWorkflow(repositoryRoot string, unit releaseconfig.ReleaseUnit) (releaseworkflow.ConsumerWorkflowFacts, *commandFailure) {
	content, err := os.ReadFile(filepath.Join(repositoryRoot, filepath.FromSlash(unit.Workflow)))
	if err != nil {
		return releaseworkflow.ConsumerWorkflowFacts{}, &commandFailure{
			Code: "PIPELINE_WORKFLOW_INVALID", Message: "The configured consumer workflow could not be read safely.",
		}
	}
	facts, err := releaseworkflow.InspectConsumerWorkflow(content, unit.IsPlugin)
	if err != nil {
		return releaseworkflow.ConsumerWorkflowFacts{}, &commandFailure{
			Code: "PIPELINE_WORKFLOW_INVALID", Message: "The configured consumer workflow is not supported YAML.",
		}
	}
	return facts, nil
}

func pipelineConsumerStages(workflowPath string, facts releaseworkflow.ConsumerWorkflowFacts) []LifecycleStage {
	stages := make([]LifecycleStage, 0, len(facts.Operations))
	for _, operation := range facts.Operations {
		conditionalReason := ""
		if operation.PluginOnly {
			conditionalReason = "selected unit is configured as a plugin"
		}
		stages = append(stages, LifecycleStage{
			ID: string(operation.ID), Label: operation.Label,
			Owner: StageOwner(operation.Owner), Location: StageLocation(operation.Location), Mutation: MutationClass(operation.Mutation),
			ConfigurationStatus: "configured", Source: workflowPath,
			ConditionalReason: conditionalReason,
		})
	}
	return stages
}

func configuredMaterializedFiles(unit releaseconfig.ReleaseUnit, identity releasetool.Identity) []pipelineMaterializedFile {
	files := make([]pipelineMaterializedFile, 0)
	if unit.IsPlugin && identity == releasetool.GoReleaser && unit.PluginManifestPath != "" {
		files = append(files, pipelineMaterializedFile{
			Path: unit.PluginManifestPath, Reason: "synchronize configured plugin manifest version during release execution",
		})
	}
	if identity == releasetool.JReleaser {
		files = append(files, pipelineMaterializedFile{
			Path:   path.Join(unit.WorkingDirectory, releasetool.JReleaserConfigFile),
			Reason: "synchronize JReleaser project version during release execution",
		})
	}
	return files
}

func configuredPublicationStatus(facts releaseworkflow.ConsumerWorkflowFacts) string {
	if releaseworkflow.HasConsumerOperation(facts, releaseworkflow.ConsumerReleasePublication) {
		return "configured"
	}
	return "not_configured"
}

func configuredPluginRegistryStatus(isPlugin bool, facts releaseworkflow.ConsumerWorkflowFacts) string {
	if !isPlugin {
		return "not_applicable"
	}
	if releaseworkflow.HasConsumerOperation(facts, releaseworkflow.ConsumerPluginIndexGeneration) &&
		releaseworkflow.HasConsumerOperation(facts, releaseworkflow.ConsumerPluginIndexPublication) {
		return "configured"
	}
	return "not_configured"
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
