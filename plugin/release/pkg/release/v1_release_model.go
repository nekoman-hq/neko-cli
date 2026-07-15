//nolint:staticcheck // This file is the explicit boundary for deprecated V1 compatibility data.
package release

import releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"

type V1ReleaseIntent struct {
	RepositoryRoot string
	Unit           releaseconfig.ReleaseUnit
	Config         *releaseconfig.V1ReleaseConfig
	ReleaseType    Type
}

type V1ReleasePreviewRequest struct {
	Intent V1ReleaseIntent
}

type V1ReleaseExecutionRequest struct {
	Intent V1ReleaseIntent
}

//nolint:govet // Logical plan order keeps version and ownership facts together.
type V1ReleasePlan struct {
	RepositoryRoot    string
	UnitID            string
	CurrentVersion    string
	LatestVersion     string
	NextVersion       string
	Tag               string
	CommitMessage     string
	ConfigFile        string
	Executor          string
	ReleaseType       Type
	IgnoredLatestTag  string
	materializedFiles []string
}

func (plan V1ReleasePlan) MaterializedFiles() []string {
	return append([]string(nil), plan.materializedFiles...)
}

type V1ExecutorRequest struct {
	Plan V1ReleasePlan
}

type V1ReleaseResult interface {
	ReleaseCommandOutcome
	v1ReleaseResult()
}

type V1ReleaseFailureClass string

const (
	v1ReleasePlanningFailure           V1ReleaseFailureClass = "planning"
	v1ReleaseRequirementsFailure       V1ReleaseFailureClass = "requirements"
	v1ReleasePreflightFailure          V1ReleaseFailureClass = "preflight"
	v1ReleaseExecutorResolutionFailure V1ReleaseFailureClass = "executor-resolution"
	v1ReleaseExecutionFailure          V1ReleaseFailureClass = "execution"
	v1ReleaseRollbackFailure           V1ReleaseFailureClass = "rollback"
)

type v1ReleaseFailureBoundary uint8

const (
	v1ReleaseStructuredFailure v1ReleaseFailureBoundary = iota + 1
	v1ReleaseFatalFailure
)

type V1ReleaseFailure struct {
	Cause    error
	Class    V1ReleaseFailureClass
	Code     string
	Boundary v1ReleaseFailureBoundary
}

func newV1ReleaseFailure(class V1ReleaseFailureClass, code string, cause error) *V1ReleaseFailure {
	return &V1ReleaseFailure{Class: class, Code: code, Cause: cause, Boundary: v1ReleaseStructuredFailure}
}

func newFatalV1ReleaseFailure(code string, cause error) *V1ReleaseFailure {
	return &V1ReleaseFailure{Class: v1ReleasePreflightFailure, Code: code, Cause: cause, Boundary: v1ReleaseFatalFailure}
}

func commandFailureFromV1(failure *V1ReleaseFailure) *CommandFailure {
	if failure == nil {
		return nil
	}
	boundary := CommandFailureStructured
	if failure.Boundary == v1ReleaseFatalFailure {
		boundary = CommandFailureFatal
	}
	return &CommandFailure{
		Cause:    failure.Cause,
		Code:     failure.Code,
		Boundary: boundary,
	}
}
