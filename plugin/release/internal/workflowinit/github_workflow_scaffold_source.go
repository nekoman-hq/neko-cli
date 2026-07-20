package workflowinit

import (
	"os"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type githubWorkflowScaffoldSourceReader interface {
	Read(string) (*releaseconfig.ReleaseRepository, *commandFailure)
}

type filesystemGitHubWorkflowScaffoldSourceReader struct{}

func (filesystemGitHubWorkflowScaffoldSourceReader) Read(root string) (*releaseconfig.ReleaseRepository, *commandFailure) {
	configPresent, err := workflowSourcePathPresent(releaseconfig.V2ConfigPath(root))
	if err != nil {
		return nil, failureFromMessage("V2_WORKFLOW_SOURCE_INVALID", "V2 release configuration could not be inspected")
	}
	statePresent, err := workflowSourcePathPresent(releaseconfig.V2StatePath(root))
	if err != nil {
		return nil, failureFromMessage("V2_WORKFLOW_SOURCE_INVALID", "V2 release state could not be inspected")
	}
	v1Present, err := workflowSourcePathPresent(filepath.Join(root, releaseconfig.V1FileName)) //nolint:staticcheck
	if err != nil {
		return nil, failureFromMessage("V2_WORKFLOW_SOURCE_INVALID", "release source files could not be inspected")
	}
	if !configPresent && !statePresent && v1Present {
		return nil, failureFromMessage("UNSUPPORTED_RELEASE_SOURCE", "GitHub Actions workflow scaffolding requires Release V2 config and state; V1-only repositories are unsupported")
	}
	if !configPresent || !statePresent {
		return nil, failureFromMessage("V2_WORKFLOW_SOURCE_MISSING", "GitHub Actions workflow scaffolding requires both .neko/release.config.json and .neko/release.state.json")
	}
	if v1Present {
		return nil, failureFromMessage("V2_WORKFLOW_SOURCE_CONFLICT", "V1 and V2 release sources conflict at the repository root")
	}
	if recoveryErr := releaseconfig.ValidateV2PairRecoveryReadiness(root); recoveryErr != nil {
		return nil, failureFromMessage("V2_WORKFLOW_RECOVERY_BLOCKED", "unresolved V2 pair recovery evidence blocks workflow scaffolding; inspect evidence before continuing")
	}

	config, err := releaseconfig.LoadV2Config(releaseconfig.V2ConfigPath(root))
	if err != nil {
		return nil, failureFromMessage("V2_WORKFLOW_CONFIGURATION_INVALID", ".neko/release.config.json is malformed or unreadable")
	}
	state, err := releaseconfig.LoadV2State(releaseconfig.V2StatePath(root))
	if err != nil {
		return nil, failureFromMessage("V2_WORKFLOW_STATE_INVALID", ".neko/release.state.json is malformed or unreadable")
	}
	if !hasConfiguredGitHubWorkflow(config) {
		return nil, failureFromMessage("WORKFLOW_NOT_CONFIGURED", "Release V2 config does not define a GitHub Actions workflow path")
	}
	if err := releaseconfig.ValidateV2ConfigStateStructure(config, state); err != nil {
		if releaseconfig.IsV2ConfigStateAlignmentError(err) {
			return nil, failureFromMessage("V2_WORKFLOW_CONFIG_STATE_MISMATCH", "V2 config and state release-unit identities do not match")
		}
		return nil, failureFromMessage("V2_WORKFLOW_SOURCE_INVALID", "the V2 config/state pair is invalid for workflow scaffolding")
	}
	return releaseconfig.NormalizeV2Repository(root, config, state), nil
}

func workflowSourcePathPresent(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func hasConfiguredGitHubWorkflow(config *releaseconfig.V2ReleaseConfig) bool {
	if config == nil {
		return false
	}
	for _, unit := range config.Units {
		if strings.TrimSpace(unit.Executor.Workflow) != "" {
			return true
		}
	}
	return false
}
