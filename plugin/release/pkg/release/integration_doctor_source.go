package release

import (
	"fmt"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type integrationDoctorSourceSnapshot struct {
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

type integrationDoctorSourceReader interface {
	Read(string) integrationDoctorSourceSnapshot
}

type filesystemIntegrationDoctorSourceReader struct{}

func (filesystemIntegrationDoctorSourceReader) Read(root string) integrationDoctorSourceSnapshot {
	snapshot := integrationDoctorSourceSnapshot{RecoveryReady: true}
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

type integrationDoctorSourceInspection struct {
	Repository  *releaseconfig.ReleaseRepository
	Diagnostics []integrationDoctorDiagnostic
}

func inspectIntegrationDoctorSource(root string, snapshot integrationDoctorSourceSnapshot) integrationDoctorSourceInspection {
	inspection := integrationDoctorSourceInspection{}
	add := func(code, message, remediation string) {
		inspection.Diagnostics = append(inspection.Diagnostics,
			newIntegrationDoctorDiagnostic(integrationDoctorError, "source", code, message, remediation))
	}
	if snapshot.InspectionErr != nil {
		add("SOURCE_INSPECTION_FAILED", "Release source files could not be inspected safely.", "Restore local read access to the repository root and source files, then run the doctor again.")
		return inspection
	}
	if snapshot.V1Present && (snapshot.ConfigPresent || snapshot.StatePresent) {
		add("MIXED_RELEASE_SOURCES", "V1 and V2 release sources coexist at the repository root.", "Remove the obsolete V1 source after completing and verifying V2 migration.")
		return inspection
	}
	if snapshot.V1Present && !snapshot.ConfigPresent && !snapshot.StatePresent {
		add("V1_SOURCE_UNSUPPORTED", "The repository contains only the deprecated V1 release source.", "Migrate the repository to the V2 config/state pair before using GitHub Actions delivery.")
		return inspection
	}
	if !snapshot.ConfigPresent {
		add("V2_CONFIG_MISSING", ".neko/release.config.json is missing.", "Create the canonical Release V2 configuration.")
	}
	if !snapshot.StatePresent {
		add("V2_STATE_MISSING", ".neko/release.state.json is missing.", "Create the canonical Release V2 state file aligned with the configuration.")
	}
	if !snapshot.ConfigPresent || !snapshot.StatePresent {
		return inspection
	}
	if snapshot.ConfigError != nil {
		add("V2_CONFIG_INVALID", ".neko/release.config.json is malformed, unreadable, or not strict JSON.", "Repair the V2 configuration as strict JSON using the canonical schema.")
	}
	if snapshot.StateError != nil {
		add("V2_STATE_INVALID", ".neko/release.state.json is malformed, unreadable, or not strict JSON.", "Repair the V2 state as strict JSON using the canonical schema.")
	}
	if snapshot.ConfigError != nil || snapshot.StateError != nil {
		return inspection
	}
	if !snapshot.RecoveryReady {
		add("V2_RECOVERY_BLOCKED", "Unresolved V2 pair recovery evidence makes the config/state pair uncertain.", "Inspect and resolve the local V2 recovery evidence before release delivery.")
	}
	if err := releaseconfig.ValidateV2ConfigStateStructure(snapshot.Config, snapshot.State); err != nil {
		code, message := classifyIntegrationDoctorSourceError(err)
		add(code, message, "Repair the V2 config/state pair, then run the doctor again.")
		return inspection
	}
	if !snapshot.RecoveryReady {
		return inspection
	}
	inspection.Repository = releaseconfig.NormalizeV2Repository(root, snapshot.Config, snapshot.State)
	return inspection
}

func classifyIntegrationDoctorSourceError(err error) (string, string) {
	if releaseconfig.IsV2ConfigStateAlignmentError(err) {
		return "V2_CONFIG_STATE_MISMATCH", "V2 config and state release-unit identities do not match."
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "tag prefix"):
		return "TAG_PREFIX_CONFLICT", "Release units do not have distinct canonical tag prefixes."
	case strings.Contains(message, "delivery"):
		return "DELIVERY_INVALID", "A Release V2 unit configures unsupported delivery instead of GitHub Actions."
	case strings.Contains(message, "executor"):
		return "EXECUTOR_INVALID", "A Release V2 unit configures an unsupported or incomplete executor."
	case strings.Contains(message, "workflow"):
		return "WORKFLOW_CONFIGURATION_INVALID", "A Release V2 unit configures a non-canonical GitHub Actions workflow path."
	case strings.Contains(message, "schemaVersion"):
		return "V2_SCHEMA_VERSION_INVALID", "The V2 config/state schema version is not exactly 2."
	case strings.Contains(message, "version"):
		return "UNIT_VERSION_INVALID", "A Release V2 state version is not canonical semantic versioning."
	default:
		return "V2_SOURCE_INVALID", fmt.Sprintf("The V2 config/state pair is structurally invalid: %s", message)
	}
}

var _ integrationDoctorSourceReader = filesystemIntegrationDoctorSourceReader{}
