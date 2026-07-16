package evidence

const (
	FamilyReleaseExecution = "release-execution"
	FamilyDispatch         = "dispatch"
	FamilyMigration        = "migration"
	FamilyV1Compensation   = "v1-compensation"
	FamilyV2PairRecovery   = "v2-pair-recovery"
)

const (
	ClassificationActive                 = "active"
	ClassificationResumable              = "resumable"
	ClassificationCompleted              = "completed"
	ClassificationTerminal               = "terminal"
	ClassificationUncertain              = "uncertain"
	ClassificationCorrupt                = "corrupt"
	ClassificationConflicting            = "conflicting"
	ClassificationManualRecoveryRequired = "manual-recovery-required"
	ClassificationUnsupported            = "unsupported"
)

// EvidenceRecord is the redacted operator summary for one known evidence file.
//
//nolint:govet // Field order follows rendered output order.
type EvidenceRecord struct {
	CreatedAt             string
	UpdatedAt             string
	Family                string
	Identity              string
	Owner                 string
	Unit                  string
	Version               string
	Tag                   string
	State                 string
	PendingAction         string
	Classification        string
	SafeToResume          bool
	AutomaticContinuation bool
	ManualRecovery        bool
	LifecycleAllowed      bool
	LifecycleOperation    string
	Guidance              string
	Path                  string
	DigestSHA256          string
}

type EvidenceDiagnostic struct {
	Family         string
	Path           string
	Classification string
	Code           string
	Guidance       string
}

type evidenceQueryResult struct {
	Records     []EvidenceRecord
	Diagnostics []EvidenceDiagnostic
}
