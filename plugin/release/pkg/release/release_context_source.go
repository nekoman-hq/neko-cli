package release

import (
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type filesystemReleaseContextSourceReader struct{}

func (filesystemReleaseContextSourceReader) ReadV2(root string) (*releaseconfig.ReleaseRepository, *CommandFailure) {
	configPresent, inspectErr := releaseContextPathPresent(releaseconfig.V2ConfigPath(root))
	if inspectErr != nil {
		return nil, failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "V2 release configuration could not be inspected")
	}
	statePresent, inspectErr := releaseContextPathPresent(releaseconfig.V2StatePath(root))
	if inspectErr != nil {
		return nil, failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "V2 release state could not be inspected")
	}
	// The validator detects—but never loads—the deprecated V1 source so that it
	// can reject ambiguous or unsupported repositories explicitly.
	v1Present, inspectErr := releaseContextPathPresent(filepath.Join(root, releaseconfig.V1FileName)) //nolint:staticcheck
	if inspectErr != nil {
		return nil, failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "release source files could not be inspected")
	}
	if !configPresent && !statePresent && v1Present {
		return nil, failureFromMessage("UNSUPPORTED_RELEASE_SOURCE", "release context validation requires V2 config and state; V1-only repositories are unsupported")
	}
	if !configPresent || !statePresent {
		return nil, failureFromMessage("V2_CONTEXT_SOURCE_MISSING", "release context validation requires both .neko/release.config.json and .neko/release.state.json")
	}
	if v1Present {
		return nil, failureFromMessage("V2_CONTEXT_SOURCE_CONFLICT", "V1 and V2 release sources conflict at the repository root")
	}
	if err := releaseconfig.ValidateV2PairRecoveryReadiness(root); err != nil {
		return nil, failureFromMessage("V2_CONTEXT_RECOVERY_BLOCKED", "unresolved V2 pair recovery evidence blocks release context validation; inspect evidence before continuing")
	}

	cfg, err := releaseconfig.LoadV2Config(releaseconfig.V2ConfigPath(root))
	if err != nil {
		return nil, failureFromMessage("V2_CONFIGURATION_INVALID", ".neko/release.config.json is malformed or unreadable")
	}
	state, err := releaseconfig.LoadV2State(releaseconfig.V2StatePath(root))
	if err != nil {
		return nil, failureFromMessage("V2_STATE_INVALID", ".neko/release.state.json is malformed or unreadable")
	}
	if err := releaseconfig.ValidateV2(root, cfg, state); err != nil {
		if releaseconfig.IsV2ConfigStateAlignmentError(err) {
			return nil, failureFromMessage("V2_CONFIG_STATE_MISMATCH", "V2 config and state release-unit identities do not match")
		}
		return nil, failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "the V2 config/state pair is invalid for release context validation")
	}
	return releaseconfig.NormalizeV2Repository(root, cfg, state), nil
}

func releaseContextPathPresent(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
