package releasesource

import (
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// Snapshot records tolerant, read-only observations about Release V2 source
// files without applying strict repository validation.
type Snapshot struct {
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

// Reader supplies tolerant Release V2 source observations.
type Reader interface {
	Read(string) Snapshot
}

// FilesystemReader observes Release V2 source files without mutation.
type FilesystemReader struct{}

func (FilesystemReader) Read(root string) Snapshot {
	snapshot := Snapshot{RecoveryReady: true}
	var err error
	snapshot.ConfigPresent, err = pathPresent(releaseconfig.V2ConfigPath(root))
	if err != nil {
		snapshot.InspectionErr = err
		return snapshot
	}
	snapshot.StatePresent, err = pathPresent(releaseconfig.V2StatePath(root))
	if err != nil {
		snapshot.InspectionErr = err
		return snapshot
	}
	snapshot.V1Present, err = pathPresent(filepath.Join(root, releaseconfig.V1FileName)) //nolint:staticcheck
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

func pathPresent(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

var _ Reader = FilesystemReader{}
