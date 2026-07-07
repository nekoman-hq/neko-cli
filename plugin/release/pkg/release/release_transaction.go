package release

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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

type ReleasePlan struct {
	UnitID             string
	CurrentVersion     string
	NextVersion        string
	Tag                string
	StateChange        string
	OwnershipSummary   string
	V2GitOwnership     string
	StateGuarantee     string
	V2LocalBlockReason string
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
	ValidateRequirements(ctx *ReleaseExecutionContext) error
	ResolveFiles(ctx *ReleaseExecutionContext) ([]string, error)
	Execute(ctx *ReleaseExecutionContext) error
}

//nolint:govet // Transaction fields follow lifecycle order rather than memory layout.
type ReleaseTransaction struct {
	Context                 *ReleaseExecutionContext
	Plan                    ReleasePlan
	MaterializationPlan     *MaterializationPlan
	Materialization         *MaterializationTransaction
	State                   *StateTransaction
	Executor                transactionExecutor
	Tracker                 *MutationTracker
	AfterMaterialization    func(*ReleaseTransaction) error
	AfterStateWrite         func(*ReleaseTransaction) error
	AfterReleaseFilesStaged func(*ReleaseTransaction) error
}

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
		State:    NewStateTransaction(ctx.RepositoryRoot),
		Executor: executor,
		Tracker:  NewMutationTracker(),
	}, nil
}

func BuildReleasePlan(ctx *ReleaseExecutionContext) ReleasePlan {
	return ReleasePlan{
		UnitID:             ctx.Unit.ID,
		CurrentVersion:     ctx.CurrentVersion,
		NextVersion:        ctx.NextVersion,
		Tag:                ctx.Tag,
		StateChange:        fmt.Sprintf("%s: %s -> %s", ctx.Unit.ID, ctx.CurrentVersion, ctx.NextVersion),
		OwnershipSummary:   ownershipSummary(ctx.Capabilities),
		V2GitOwnership:     NewV2GitOwnership().Summary(),
		StateGuarantee:     ctx.Capabilities.StateCommitGuarantee,
		V2LocalBlockReason: ctx.Capabilities.V2LocalExecutionBlockedReason,
	}
}

func (tx *ReleaseTransaction) Execute() (*ReleaseTransactionResult, error) {
	if tx.Context.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, fmt.Errorf("release transaction supports V2 repositories only")
	}
	if tx.Context.DryRun {
		return nil, fmt.Errorf("release transaction cannot execute in dry-run mode")
	}
	return nil, fmt.Errorf("%s", v2GitCoordinationUnavailableMessage)
}

func (tx *ReleaseTransaction) prepareReleaseFilesForCoordinator() (*KnownReleaseFiles, error) {
	if err := validateV2LocalExecution(tx.Context); err != nil {
		return nil, err
	}
	if err := tx.Executor.ValidateRequirements(tx.Context); err != nil {
		return nil, err
	}
	files, err := tx.Executor.ResolveFiles(tx.Context)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		tx.Tracker.TrackFile(filepath.Join(tx.Context.UnitRoot, file))
	}
	if tx.Context.Capabilities.RequiresRepositoryCleanliness {
		if cleanErr := ensureGitClean(tx.Context.RepositoryRoot); cleanErr != nil {
			return nil, cleanErr
		}
	}
	tx.Tracker.Mark(ExecutionPhasePreflightValidated)

	materializer, err := ResolveVersionMaterializer(tx.Context.Executor)
	if err != nil {
		return nil, err
	}
	materializationPlan, err := materializer.Plan(tx.Context)
	if err != nil {
		return nil, err
	}
	if validationErr := materializer.Validate(materializationPlan); validationErr != nil {
		return nil, validationErr
	}
	tx.MaterializationPlan = materializationPlan
	tx.Materialization = NewMaterializationTransaction(materializationPlan)
	if captureErr := tx.Materialization.CaptureSnapshots(); captureErr != nil {
		return nil, captureErr
	}
	tx.Tracker.Mark(ExecutionPhaseMaterializationPrepared)
	if _, applyErr := tx.Materialization.Apply(); applyErr != nil {
		return nil, tx.fail(applyErr)
	}
	for _, file := range tx.Materialization.ChangedFiles() {
		tx.Tracker.TrackFile(file)
	}
	tx.Tracker.Mark(ExecutionPhaseMaterializationApplied)

	if tx.AfterMaterialization != nil {
		if hookErr := tx.AfterMaterialization(tx); hookErr != nil {
			return nil, tx.fail(hookErr)
		}
	}

	if stateCaptureErr := tx.State.CaptureSnapshot(); stateCaptureErr != nil {
		return nil, tx.fail(stateCaptureErr)
	}
	if stateWriteErr := tx.State.WriteUnitVersion(tx.Context.Unit.ID, tx.Context.NextVersion); stateWriteErr != nil {
		return nil, tx.fail(stateWriteErr)
	}
	tx.Tracker.TrackFile(tx.State.StatePath)
	tx.Tracker.Mark(ExecutionPhaseStatePrepared)

	if tx.AfterStateWrite != nil {
		if hookErr := tx.AfterStateWrite(tx); hookErr != nil {
			return nil, tx.fail(hookErr)
		}
	}
	knownFiles, err := NewKnownReleaseFiles(tx.Context, tx.MaterializationPlan)
	if err != nil {
		return nil, tx.fail(err)
	}
	return &knownFiles, nil
}

func (tx *ReleaseTransaction) fail(cause error) error {
	if !tx.Tracker.Irreversible {
		var restoreErrors []string
		if tx.State != nil && tx.State.Snapshot.Path != "" {
			if restoreErr := tx.State.RestoreSnapshot(); restoreErr != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("state %s: %v", tx.State.StatePath, restoreErr))
			}
		}
		if tx.Materialization != nil {
			if restoreErr := tx.Materialization.Restore(); restoreErr != nil {
				restoreErrors = append(restoreErrors, fmt.Sprintf("materialized files: %v", restoreErr))
			}
		}
		if unstageErr := unstageKnownFiles(tx.Context.RepositoryRoot, tx.Tracker.KnownStagedFiles); unstageErr != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("unstage: %v", unstageErr))
		}
		tx.Tracker.Mark(ExecutionPhaseFailed)
		if len(restoreErrors) > 0 {
			return fmt.Errorf("%w; recovery errors: %s", cause, strings.Join(restoreErrors, "; "))
		}
		return fmt.Errorf("%w; restored V2 state and materialized files from snapshots", cause)
	}
	reachedPhase := tx.Tracker.Phase
	tx.Tracker.Mark(ExecutionPhaseFailed)
	return fmt.Errorf("%w; V2 release reached phase %s for unit %q and tag %q; no destructive rollback was attempted; inspect changed files: %s",
		cause,
		reachedPhase,
		tx.Context.Unit.ID,
		tx.Context.Tag,
		strings.Join(tx.Tracker.KnownChangedFiles, ", "),
	)
}

func validateV2LocalExecution(ctx *ReleaseExecutionContext) error {
	if ctx.Delivery != string(releaseconfig.DeliveryLocal) {
		return fmt.Errorf("github-actions delivery is configured but not implemented yet")
	}
	if !ctx.DeliveryMode.SupportsLocalExecution {
		return fmt.Errorf("%s delivery is configured but local execution is not supported", ctx.Delivery)
	}
	if !ctx.Capabilities.SupportsLocalExecution {
		return fmt.Errorf("executor %s does not support local execution", ctx.Executor)
	}
	if !ctx.Capabilities.StateCommitGuaranteed {
		if ctx.Capabilities.V2LocalExecutionBlockedReason != "" {
			return errors.New(ctx.Capabilities.V2LocalExecutionBlockedReason)
		}
		return fmt.Errorf("executor %s does not guarantee that V2 state is included in the release commit", ctx.Executor)
	}
	return nil
}

func ensureGitClean(repositoryRoot string) error {
	cmd := exec.Command("git", "-C", repositoryRoot, "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to check git status: %s: %w", strings.TrimSpace(string(output)), err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("the working tree has uncommitted changes. Please commit or stash them")
	}
	return nil
}

func unstageKnownFiles(repositoryRoot string, absolutePaths []string) error {
	if len(absolutePaths) == 0 {
		return nil
	}
	paths := make([]string, 0, len(absolutePaths))
	for _, absolutePath := range absolutePaths {
		relativePath, err := filepath.Rel(repositoryRoot, absolutePath)
		if err != nil {
			return fmt.Errorf("staged path %s cannot be related to repository root: %w", absolutePath, err)
		}
		paths = append(paths, relativePath)
	}
	args := append([]string{"-C", repositoryRoot, "restore", "--staged", "--"}, paths...)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unstage release files %s: %s: %w", strings.Join(paths, ", "), strings.TrimSpace(string(output)), err)
	}
	return nil
}

func ownershipSummary(capabilities ExecutorCapabilities) string {
	return fmt.Sprintf("versionFiles=%s commit=%s tag=%s push=%s githubRelease=%s",
		capabilities.VersionFilesOwner,
		capabilities.CommitOwner,
		capabilities.TagOwner,
		capabilities.PushOwner,
		capabilities.GitHubReleaseOwner,
	)
}
