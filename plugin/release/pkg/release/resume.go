package release

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type resumableExecutionLocator interface {
	Find(unitID string) (*resumableExecution, *CommandFailure)
}

type resumableExecutionAssessor interface {
	Assess(*resumableExecution) (*ReleaseExecutionRecoveryAssessment, *CommandFailure)
}

type resumeExecutionContextReconstructor interface {
	Reconstruct(*resumableExecution) (*resumableReleaseExecution, *CommandFailure)
}

type resumeRecoveryResolver interface {
	Resolve(*ReleaseExecutionJournal, *ReleaseExecutionRecoveryAssessment) resumeRecoveryResolution
}

// resumeReleaseUseCase coordinates the four application-level responsibilities
// of resume: discovery, assessment, operation selection, and invocation.
type resumeReleaseUseCase struct {
	locator  resumableExecutionLocator
	assessor resumableExecutionAssessor
	contexts resumeExecutionContextReconstructor
	resolver resumeRecoveryResolver
	selector resumeReleaseOperationSelector
}

func (useCase resumeReleaseUseCase) Resume(ctx context.Context, request ResumeCommandRequest) (ResumeCommandOutcome, *CommandFailure) {
	execution, failure := useCase.locator.Find(request.UnitID)
	if failure != nil {
		return nil, failure
	}
	assessment, failure := useCase.assessor.Assess(execution)
	if failure != nil {
		return nil, failure
	}
	if request.DryRun {
		result, err := newResumeAssessment(execution.resolution.Path, execution.resolution.Journal, assessment)
		if err != nil {
			return nil, failureFromError("RECOVERY_ASSESSMENT_FAILED", err)
		}
		return result, nil
	}

	resolution := useCase.resolver.Resolve(execution.resolution.Journal, assessment)
	if resolution.Refusal != nil && resolution.Refusal.blocksBeforeContextReconstruction() {
		return nil, failureForResumeRecoveryRefusal(resolution.Refusal)
	}
	compatible, failure := useCase.contexts.Reconstruct(execution)
	if failure != nil {
		return nil, failure
	}
	if resolution.Refusal != nil {
		return nil, failureForResumeRecoveryRefusal(resolution.Refusal)
	}
	operation, err := useCase.selector.Select(resolution.Operation)
	if err != nil {
		return nil, failureFromError("RESUME_FAILED", err)
	}
	return operation.Resume(ctx, compatible)
}

// resumableExecution contains only discovered facts for one existing release
// execution. It is not a mutable workflow state.
type resumableExecution struct {
	repository *releaseconfig.ReleaseRepository
	unit       releaseconfig.ReleaseUnit
	resolution ReleaseExecutionJournalResolution
	remoteName string
	remoteURL  string
}

type resumeReleaseRemoteResolver interface {
	Selected(repositoryRoot string) (remoteName, remoteURL string, err error)
}

type locateResumableExecution struct {
	remotes        resumeReleaseRemoteResolver
	journal        unresolvedReleaseExecutionFinder
	repositoryPath string
}

func (locator locateResumableExecution) Find(unitID string) (*resumableExecution, *CommandFailure) {
	repository, err := releaseconfig.LoadReleaseRepository(locator.repositoryPath)
	if err != nil {
		return nil, failureFromError("CONFIG_NOT_FOUND", err)
	}
	if repository.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, failureFromMessage("RESUME_UNSUPPORTED", "release resume supports V2 github-actions releases only")
	}
	unit, err := releaseconfig.ResolveReleaseUnit(repository, unitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromError("UNIT_RESOLUTION_FAILED", err)
	}
	remoteName, remoteURL, err := locator.remotes.Selected(repository.RepositoryRoot)
	if err != nil {
		return nil, failureFromError("REMOTE_RESOLUTION_FAILED", err)
	}
	matches, err := locator.journal.FindUnresolved(remoteURL, unit.ID)
	if err != nil {
		return nil, failureFromError("JOURNAL_SCAN_FAILED", err)
	}
	if len(matches) == 0 {
		return nil, failureFromMessage("NO_RESUMABLE_JOURNAL", fmt.Sprintf("no resumable V2 release execution journal found for unit %s", unit.ID))
	}
	if len(matches) > 1 {
		return nil, failureFromMessage("MULTIPLE_RESUMABLE_JOURNALS", multipleJournalMessage(unit.ID, matches))
	}
	resolution := matches[0]
	if resolution.Journal.Delivery != string(releaseconfig.DeliveryGitHubActions) {
		return nil, failureFromMessage("RESUME_UNSUPPORTED", "release resume supports V2 github-actions releases only")
	}
	return &resumableExecution{
		repository: repository,
		unit:       *unit,
		resolution: resolution,
		remoteName: remoteName,
		remoteURL:  remoteURL,
	}, nil
}

type assessResumableExecution struct{}

func (assessResumableExecution) Assess(execution *resumableExecution) (*ReleaseExecutionRecoveryAssessment, *CommandFailure) {
	assessment, err := AssessReleaseExecutionRecovery(execution.repository.RepositoryRoot, execution.resolution.Journal)
	if err != nil {
		return nil, failureFromError("RECOVERY_ASSESSMENT_FAILED", err)
	}
	return assessment, nil
}

type reconstructResumeExecutionContext struct{}

func (reconstructResumeExecutionContext) Reconstruct(execution *resumableExecution) (*resumableReleaseExecution, *CommandFailure) {
	execCtx, err := executionContextFromJournal(execution.repository, execution.unit, execution.resolution.Journal)
	if err != nil {
		return nil, failureFromError("JOURNAL_CONTEXT_FAILED", err)
	}
	if execCtx.Workflow != execution.unit.Workflow || execCtx.Executor != execution.unit.ExecutorType || execCtx.Delivery != execution.unit.Delivery {
		return nil, failureFromMessage("JOURNAL_CONFLICT", "current V2 config no longer matches the release execution journal")
	}
	return &resumableReleaseExecution{Discovered: execution, Context: execCtx}, nil
}

type resumeReleaseGitReader interface {
	CurrentBranch(repositoryRoot string) (string, error)
	Upstream(repositoryRoot, branch string) (remote, upstreamBranch string, err error)
	TagCommit(repositoryRoot, tag string) (string, error)
}

type prepareResumeRelease struct {
	git resumeReleaseGitReader
}

func (preparer prepareResumeRelease) Prepare(execution *resumableReleaseExecution) (reconstructedResumeRelease, *CommandFailure) {
	branch, err := preparer.git.CurrentBranch(execution.Context.RepositoryRoot)
	if err != nil {
		return reconstructedResumeRelease{}, failureFromError("RESUME_FAILED", err)
	}
	_, upstreamBranch, err := preparer.git.Upstream(execution.Context.RepositoryRoot, branch)
	if err != nil {
		return reconstructedResumeRelease{}, failureFromError("RESUME_FAILED", err)
	}
	journal := execution.Discovered.resolution.Journal
	if journal.ReleaseCommitSHA == "" {
		return reconstructedResumeRelease{}, failureFromError("RESUME_FAILED", fmt.Errorf("resume before release commit is not yet safe for automatic continuation; use --dry-run and inspect recovery guidance"))
	}
	return reconstructedResumeRelease{
		Context: execution.Context,
		Execution: preparedGitHubActionsReleaseExecution{
			Identity: journal.Identity,
			Path:     execution.Discovered.resolution.Path,
		},
		Preflight: validatedGitHubActionsReleasePreflight{
			Git: GitReleasePreflight{
				Branch:         branch,
				Remote:         execution.Discovered.remoteName,
				UpstreamBranch: upstreamBranch,
			},
			RemoteURL:     execution.Discovered.remoteURL,
			BaseCommitSHA: journal.BaseCommitSHA,
		},
		Files:     knownReleaseFilesFromJournal(execution.Context.RepositoryRoot, journal),
		CommitSHA: journal.ReleaseCommitSHA,
	}, nil
}

func knownReleaseFilesFromJournal(repositoryRoot string, journal *ReleaseExecutionJournal) KnownReleaseFiles {
	files := KnownReleaseFiles{RepositoryRoot: repositoryRoot, Files: make([]KnownReleaseFile, 0, len(journal.KnownReleaseFiles))}
	for _, file := range journal.KnownReleaseFiles {
		files.Files = append(files.Files, KnownReleaseFile{
			AbsolutePath:             filepath.Join(repositoryRoot, filepath.FromSlash(file.RepositoryRelativePath)),
			RepositoryRelativePath:   file.RepositoryRelativePath,
			ExpectedExistsBefore:     file.ExpectedExistsBefore,
			ExpectedExistsAfter:      file.ExpectedExistsAfter,
			PreimageSHA256:           file.PreimageSHA256,
			PostimageSHA256:          file.PostimageSHA256,
			RequiredForReleaseCommit: file.RequiredForReleaseCommit,
			Reason:                   file.Reason,
		})
	}
	return files
}

type resumeGitAdapter struct {
	coordinator *GitReleaseCoordinator
}

func (adapter resumeGitAdapter) Selected(repositoryRoot string) (string, string, error) {
	branch, err := adapter.CurrentBranch(repositoryRoot)
	if err != nil {
		return "", "", err
	}
	remoteName, _, err := adapter.Upstream(repositoryRoot, branch)
	if err != nil {
		return "", "", err
	}
	remoteURL, err := adapter.coordinator.gitOutput(repositoryRoot, "remote", "get-url", remoteName)
	if err != nil {
		return "", "", err
	}
	return remoteName, strings.TrimSpace(remoteURL), nil
}

func (adapter resumeGitAdapter) CurrentBranch(repositoryRoot string) (string, error) {
	return adapter.coordinator.currentBranch(repositoryRoot)
}

func (adapter resumeGitAdapter) Upstream(repositoryRoot, branch string) (string, string, error) {
	return adapter.coordinator.upstream(repositoryRoot, branch)
}

func (adapter resumeGitAdapter) TagCommit(repositoryRoot, tag string) (string, error) {
	return adapter.coordinator.tagCommit(repositoryRoot, tag)
}

type explicitResumeRecoveryResolver struct{}

func (explicitResumeRecoveryResolver) Resolve(journal *ReleaseExecutionJournal, assessment *ReleaseExecutionRecoveryAssessment) resumeRecoveryResolution {
	return resolveResumeRecovery(journal, assessment)
}

type resumeReleaseOperationSelector struct {
	fromCommitCreated resumeReleaseExecutionOperation
	fromTagCreated    resumeReleaseExecutionOperation
	fromTagPushed     resumeReleaseExecutionOperation
	completedHandoff  resumeReleaseExecutionOperation
}

func (selector resumeReleaseOperationSelector) Select(kind resumeReleaseOperationKind) (resumeReleaseExecutionOperation, error) {
	switch kind {
	case resumeReleaseFromCommitCreated:
		return selector.fromCommitCreated, nil
	case resumeReleaseFromTagCreated:
		return selector.fromTagCreated, nil
	case resumeReleaseFromTagPushed:
		return selector.fromTagPushed, nil
	case returnCompletedReleaseHandoff:
		return selector.completedHandoff, nil
	default:
		return nil, fmt.Errorf("resume release operation %d is unsupported", kind)
	}
}

func newResumeReleaseUseCase(repositoryPath string) resumeReleaseUseCase {
	runner := NewGitHubActionsReleaseRunner()
	executionJournal := NewReleaseExecutionJournalStore(repositoryPath)
	dispatchJournal := NewDispatchJournalStore(repositoryPath)
	activeGit := githubActionsReleaseGitAdapter{coordinator: runner.coordinator}
	resumeGit := resumeGitAdapter{coordinator: runner.coordinator}
	preparer := prepareResumeRelease{git: resumeGit}
	dispatchAssessor := assessGitHubActionsResumeDispatch{journal: dispatchJournal, requests: verifiedReleaseDispatchRequestBuilder{}}
	handoff := confirmGitHubActionsReleaseHandoff{journal: executionJournal}
	dispatchSelector := resumeDispatchOperationSelector{
		fresh: requestFreshGitHubActionsResumeDispatch{
			tokens: runner.tokenResolver,
			dispatcher: dispatchGitHubActionsReleaseWorkflow{
				repositoryRoot: repositoryPath,
				client:         runner.dispatchClient,
				journal:        executionJournal,
			},
			handoff: handoff,
		},
		accepted: reuseAcceptedGitHubActionsResumeDispatch{handoff: handoff},
	}
	fromTagPushed := &resumeFromTagPushedOperation{
		preparer:   preparer,
		dispatches: dispatchAssessor,
		selector:   dispatchSelector,
	}
	fromTagCreated := &resumeFromTagCreatedOperation{
		preparer:         preparer,
		dispatches:       dispatchAssessor,
		dispatchPreparer: prepareGitHubActionsReleaseDispatch{journal: executionJournal, dispatch: dispatchJournal, requests: verifiedReleaseDispatchRequestBuilder{}},
		commitPusher:     pushGitHubActionsReleaseCommit{journal: executionJournal, git: activeGit},
		tagPusher:        pushGitHubActionsReleaseTag{journal: executionJournal, git: activeGit},
		continuation:     fromTagPushed,
	}
	return resumeReleaseUseCase{
		locator:  locateResumableExecution{repositoryPath: repositoryPath, remotes: resumeGit, journal: executionJournal},
		assessor: assessResumableExecution{},
		contexts: reconstructResumeExecutionContext{},
		resolver: explicitResumeRecoveryResolver{},
		selector: resumeReleaseOperationSelector{
			fromCommitCreated: &resumeFromCommitCreatedOperation{
				preparer:     preparer,
				tags:         resumeGit,
				creator:      createGitHubActionsReleaseTag{journal: executionJournal, git: activeGit},
				continuation: fromTagCreated,
			},
			fromTagCreated:   fromTagCreated,
			fromTagPushed:    fromTagPushed,
			completedHandoff: returnCompletedReleaseHandoffOperation{},
		},
	}
}

func (refusal *resumeRecoveryRefusal) blocksBeforeContextReconstruction() bool {
	return refusal.Kind == resumeRecoveryRefusalConflicted ||
		refusal.Kind == resumeRecoveryRefusalCorrupted ||
		refusal.Kind == resumeRecoveryRefusalAmbiguousCommitPush ||
		refusal.Kind == resumeRecoveryRefusalAmbiguousTagPush
}

func failureForResumeRecoveryRefusal(refusal *resumeRecoveryRefusal) *CommandFailure {
	switch refusal.Kind {
	case resumeRecoveryRefusalConflicted, resumeRecoveryRefusalCorrupted:
		return failureFromMessage("RESUME_BLOCKED", refusal.Guidance)
	case resumeRecoveryRefusalAmbiguousCommitPush, resumeRecoveryRefusalAmbiguousTagPush:
		return failureFromMessage("RESUME_BLOCKED", "pending push action is ambiguous; manual verification is required before retry")
	case resumeRecoveryRefusalBeforeCommit:
		return failureFromError("RESUME_FAILED", fmt.Errorf("resume before release commit is not yet safe for automatic continuation; use --dry-run and inspect recovery guidance"))
	case resumeRecoveryRefusalUnprovenCommitPush, resumeRecoveryRefusalUnprovenTagPush:
		return failureFromError("RESUME_FAILED", fmt.Errorf("resume cannot prove push completion for state %s; manual verification is required", refusal.State))
	case resumeRecoveryRefusalUnsupportedState:
		return failureFromError("RESUME_FAILED", fmt.Errorf("resume from state %s requires manual inspection before continuing", refusal.State))
	default:
		return failureFromError("RESUME_FAILED", fmt.Errorf("resume recovery refusal %d is unsupported", refusal.Kind))
	}
}

func executionContextFromJournal(repository *releaseconfig.ReleaseRepository, unit releaseconfig.ReleaseUnit, journal *ReleaseExecutionJournal) (*ReleaseExecutionContext, error) {
	repositoryRoot, err := absoluteExistingDir(repository.RepositoryRoot, "repository root")
	if err != nil {
		return nil, err
	}
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return nil, err
	}
	unitRoot, err := resolveUnitRoot(repositoryRoot, repository.SourceFormat, unit)
	if err != nil {
		return nil, err
	}
	delivery, err := ResolveDelivery(unit.Delivery)
	if err != nil {
		return nil, err
	}
	capabilities, err := ResolveExecutorCapabilities(unit.ExecutorType)
	if err != nil {
		return nil, err
	}
	return &ReleaseExecutionContext{
		RepositoryRoot: repositoryRoot,
		Unit:           unit,
		UnitRoot:       unitRoot,
		CurrentVersion: journal.CurrentVersion,
		NextVersion:    journal.NextVersion,
		Tag:            journal.Tag,
		TagSpec:        tagSpec,
		ReleaseKind:    Patch,
		Executor:       journal.Executor,
		Delivery:       journal.Delivery,
		Workflow:       journal.WorkflowPath,
		SourceFormat:   releaseconfig.SourceFormatV2,
		Capabilities:   capabilities,
		DeliveryMode:   delivery,
	}, nil
}

func multipleJournalMessage(unitID string, matches []ReleaseExecutionJournalResolution) string {
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.Journal == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s version=%s tag=%s state=%s pending=%s", match.Path, match.Journal.NextVersion, match.Journal.Tag, match.Journal.State, match.Journal.PendingAction))
	}
	return fmt.Sprintf("multiple resumable release journals found for unit %s; manual inspection is required: %s", unitID, strings.Join(parts, "; "))
}
