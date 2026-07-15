package release

import "fmt"

type CommandFailureBoundary uint8

const (
	CommandFailureStructured CommandFailureBoundary = iota
	CommandFailureFatal
)

// CommandFailure is an expected command failure with a stable public code.
// Cause is retained for application diagnostics while response mapping exposes
// only its message and explicitly supplied details.
type CommandFailure struct {
	Cause    error
	Details  map[string]any
	Code     string
	Message  string
	Boundary CommandFailureBoundary
}

func failureFromError(code string, cause error) *CommandFailure {
	return &CommandFailure{Code: code, Cause: cause}
}

func failureFromMessage(code, message string) *CommandFailure {
	return &CommandFailure{Code: code, Message: message}
}

func (failure *CommandFailure) responseMessage() string {
	if failure == nil {
		return ""
	}
	if failure.Cause != nil {
		return failure.Cause.Error()
	}
	return failure.Message
}

// FatalCommandError carries the historical V1 fatal-preflight contract from
// the command handler to the plugin process boundary without exiting inside
// application orchestration.
type FatalCommandError struct {
	failure *CommandFailure
}

func (fatal *FatalCommandError) Error() string {
	if fatal == nil || fatal.failure == nil {
		return ""
	}
	return fatal.failure.responseMessage()
}

func (fatal *FatalCommandError) Code() string {
	if fatal == nil || fatal.failure == nil {
		return ""
	}
	return fatal.failure.Code
}

// ReleaseCommandOutcome seals the result variants understood by the release
// presentation mapper without introducing a generic result framework.
type ReleaseCommandOutcome interface {
	releaseCommandOutcome()
}

type V1ReleasePreview struct {
	ReleaseType    Type
	CurrentVersion string
	NextVersion    string
	ReleaseSystem  string
}

func (*V1ReleasePreview) releaseCommandOutcome() {}
func (*V1ReleasePreview) v1ReleaseResult()       {}

// LegacyReleasePreview preserves the public compatibility name while the
// active application path uses the explicitly owned V1 result.
type LegacyReleasePreview = V1ReleasePreview

type V1ReleaseCompleted struct {
	ReleaseType     Type
	PreviousVersion string
	NextVersion     string
	ReleaseSystem   string
}

func (*V1ReleaseCompleted) releaseCommandOutcome() {}
func (*V1ReleaseCompleted) v1ReleaseResult()       {}

// LegacyReleaseCompleted preserves the public compatibility name while the
// active application path uses the explicitly owned V1 result.
type LegacyReleaseCompleted = V1ReleaseCompleted

// V2ReleasePreview contains the application facts required to render the
// existing V2 dry-run contract. It deliberately contains no plugin response
// concepts.
//
//nolint:govet // Fields follow the stable response order.
type V2ReleasePreview struct {
	UnitID                       string
	CurrentVersion               string
	NextVersion                  string
	Tag                          string
	Executor                     string
	Delivery                     string
	Workflow                     string
	WorkingDirectory             string
	UnitRoot                     string
	StateChange                  string
	MaterializedFilePaths        []string
	MaterializationBlockedReason string
	KnownReleaseFilePaths        []string
	CommitMessage                string
	OwnershipSummary             string
	V2GitOwnership               string
	StateGuarantee               string
	Dispatch                     *ReleaseDispatchDryRunSummary
}

func (*V2ReleasePreview) releaseCommandOutcome() {}

func newV2ReleasePreview(
	execCtx *ReleaseExecutionContext,
	plan ReleasePlan,
	materializationPlan *MaterializationPlan,
	knownFiles KnownReleaseFiles,
	dispatch *ReleaseDispatchDryRunSummary,
) *V2ReleasePreview {
	paths := make([]string, 0)
	blockedReason := ""
	if materializationPlan != nil {
		paths = make([]string, 0, len(materializationPlan.Changes))
		for _, change := range materializationPlan.Changes {
			paths = append(paths, change.RepositoryRelativePath)
		}
		blockedReason = materializationPlan.BlockedReason
	}
	return &V2ReleasePreview{
		UnitID:                       execCtx.Unit.ID,
		CurrentVersion:               execCtx.CurrentVersion,
		NextVersion:                  execCtx.NextVersion,
		Tag:                          execCtx.Tag,
		Executor:                     execCtx.Executor,
		Delivery:                     execCtx.Delivery,
		Workflow:                     execCtx.Workflow,
		WorkingDirectory:             execCtx.Unit.WorkingDirectory,
		UnitRoot:                     execCtx.UnitRoot,
		StateChange:                  plan.StateChange,
		MaterializedFilePaths:        paths,
		MaterializationBlockedReason: blockedReason,
		KnownReleaseFilePaths:        knownFiles.RelativePaths(),
		CommitMessage:                ReleaseCommitMessage(execCtx),
		OwnershipSummary:             plan.OwnershipSummary,
		V2GitOwnership:               plan.V2GitOwnership,
		StateGuarantee:               plan.StateGuarantee,
		Dispatch:                     dispatch,
	}
}

func (*GitHubActionsReleaseResult) releaseCommandOutcome() {}

// ResumeCommandOutcome seals the result variants understood by the resume
// presentation mapper.
type ResumeCommandOutcome interface {
	resumeCommandOutcome()
}

// ResumeAssessment contains the read-only recovery facts rendered by
// `release resume --dry-run`.
//
//nolint:govet // Fields follow the stable response order.
type ResumeAssessment struct {
	UnitID               string
	NextVersion          string
	Tag                  string
	ExecutionJournalPath string
	State                ReleaseExecutionJournalState
	PendingAction        ReleaseExecutionPendingAction
	RecoveryStatus       ReleaseExecutionRecoveryStatus
	SafeToContinue       bool
	KnownFilePaths       []string
	Guidance             string
}

func (*ResumeAssessment) resumeCommandOutcome() {}

func newResumeAssessment(path string, journal *ReleaseExecutionJournal, assessment *ReleaseExecutionRecoveryAssessment) (*ResumeAssessment, error) {
	if journal == nil {
		return nil, fmt.Errorf("release execution journal is missing")
	}
	if assessment == nil {
		return nil, fmt.Errorf("release execution recovery assessment is missing")
	}
	knownFiles := make([]string, 0, len(journal.KnownReleaseFiles))
	for _, file := range journal.KnownReleaseFiles {
		knownFiles = append(knownFiles, file.RepositoryRelativePath)
	}
	return &ResumeAssessment{
		UnitID:               journal.UnitID,
		NextVersion:          journal.NextVersion,
		Tag:                  journal.Tag,
		ExecutionJournalPath: path,
		State:                journal.State,
		PendingAction:        journal.PendingAction,
		RecoveryStatus:       assessment.Status,
		SafeToContinue:       assessment.SafeToContinue,
		KnownFilePaths:       knownFiles,
		Guidance:             assessment.Guidance,
	}, nil
}

func (*GitHubActionsReleaseResult) resumeCommandOutcome() {}
