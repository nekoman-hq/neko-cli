package release

import (
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type ExecutionPhase string

const (
	ExecutionPhasePlanned                 ExecutionPhase = "planned"
	ExecutionPhasePreflightValidated      ExecutionPhase = "preflight-validated"
	ExecutionPhaseMaterializationPrepared ExecutionPhase = "materialization-prepared"
	ExecutionPhaseMaterializationApplied  ExecutionPhase = "materialization-applied"
	ExecutionPhaseStatePrepared           ExecutionPhase = "state-prepared"
	ExecutionPhaseReleaseFilesStaged      ExecutionPhase = "release-files-staged"
	ExecutionPhaseCommitOrTagStarted      ExecutionPhase = "commit-or-tag-started"
	ExecutionPhaseRemoteSideEffectStarted ExecutionPhase = "remote-side-effect-started"
	ExecutionPhaseCompleted               ExecutionPhase = "completed"
	ExecutionPhaseFailed                  ExecutionPhase = "failed"
)

type MutationTracker struct {
	Phase             ExecutionPhase
	KnownChangedFiles []string
	KnownStagedFiles  []string
	Irreversible      bool
}

func NewMutationTracker() *MutationTracker {
	return &MutationTracker{Phase: ExecutionPhasePlanned}
}

func (mt *MutationTracker) Mark(phase ExecutionPhase) {
	mt.Phase = phase
	switch phase {
	case ExecutionPhaseCommitOrTagStarted, ExecutionPhaseRemoteSideEffectStarted, ExecutionPhaseCompleted:
		mt.Irreversible = true
	}
}

func (mt *MutationTracker) TrackFile(path string) {
	for _, existing := range mt.KnownChangedFiles {
		if existing == path {
			return
		}
	}
	mt.KnownChangedFiles = append(mt.KnownChangedFiles, path)
}

func (mt *MutationTracker) TrackStagedFile(path string) {
	for _, existing := range mt.KnownStagedFiles {
		if existing == path {
			return
		}
	}
	mt.KnownStagedFiles = append(mt.KnownStagedFiles, path)
}

type ReleaseTransactionResult struct {
	Phase             ExecutionPhase
	UnitID            string
	CurrentVersion    string
	NextVersion       string
	Tag               string
	KnownChangedFiles []string
	RolledBackState   bool
}

type transactionExecutor interface {
	Name() string
}

// Deprecated: V2 local delivery is unsupported; public V2 releases use
// GitHub Actions delivery.
//
//nolint:govet // Compatibility fields keep their public shape despite layout-only alignment suggestions.
type ReleaseTransaction struct {
	Context  *ReleaseExecutionContext
	Plan     ReleasePlan
	Executor transactionExecutor
	Tracker  *MutationTracker
}

// Deprecated: V2 local delivery is unsupported; public V2 releases use
// GitHub Actions delivery.
func NewReleaseTransaction(ctx *ReleaseExecutionContext, executor transactionExecutor) (*ReleaseTransaction, error) {
	if ctx == nil {
		return nil, fmt.Errorf("release execution context is missing")
	}
	if executor == nil {
		return nil, fmt.Errorf("release executor is missing")
	}
	plan := BuildReleasePlan(ctx)
	return &ReleaseTransaction{
		Context:  ctx,
		Plan:     plan,
		Executor: executor,
		Tracker:  NewMutationTracker(),
	}, nil
}

// Execute always rejects V2 local delivery. The former private preparation path
// was removed because it could not satisfy the current V2 safety contract.
func (tx *ReleaseTransaction) Execute() (*ReleaseTransactionResult, error) {
	if tx.Context.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, fmt.Errorf("release transaction supports V2 repositories only")
	}
	if tx.Context.DryRun {
		return nil, fmt.Errorf("release transaction cannot execute in dry-run mode")
	}
	return nil, fmt.Errorf("V2 local delivery is not supported; use github-actions delivery with a workflow")
}
