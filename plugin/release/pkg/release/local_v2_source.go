package release

import (
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type localV2SourceSnapshot struct {
	Config        *releaseconfig.V2ReleaseConfig
	State         *releaseconfig.V2ReleaseState
	ConfigError   error
	StateError    error
	InspectionErr error
	ConfigPresent bool
	StatePresent  bool
	V1Present     bool
	RecoveryReady bool
}

type filesystemLocalV2SourceReader struct{}

func (filesystemLocalV2SourceReader) Read(root string) localV2SourceSnapshot {
	snapshot := localV2SourceSnapshot{RecoveryReady: true}
	var err error
	snapshot.ConfigPresent, err = releaseContextPathPresent(releaseconfig.V2ConfigPath(root))
	if err != nil {
		snapshot.InspectionErr = err
		return snapshot
	}
	snapshot.StatePresent, err = releaseContextPathPresent(releaseconfig.V2StatePath(root))
	if err != nil {
		snapshot.InspectionErr = err
		return snapshot
	}
	snapshot.V1Present, err = releaseContextPathPresent(filepath.Join(root, releaseconfig.V1FileName)) //nolint:staticcheck
	if err != nil {
		snapshot.InspectionErr = err
		return snapshot
	}
	if snapshot.ConfigPresent {
		snapshot.Config, snapshot.ConfigError = releaseconfig.LoadV2Config(releaseconfig.V2ConfigPath(root))
	}
	if snapshot.StatePresent {
		snapshot.State, snapshot.StateError = releaseconfig.LoadV2State(releaseconfig.V2StatePath(root))
	}
	if snapshot.ConfigPresent && snapshot.StatePresent {
		if err := releaseconfig.ValidateV2PairRecoveryReadiness(root); err != nil {
			snapshot.RecoveryReady = false
		}
	}
	return snapshot
}
