package release

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

// GitHubActionsReleaseRunner executes the public V2 github-actions release
// flow. Local executors still do not publish; GitHub Actions publishes from the
// pushed tag after dispatch.
type GitHubActionsReleaseRunner struct {
	coordinator    *GitReleaseCoordinator
	tokenResolver  GitHubActionsDispatchTokenResolver
	dispatchClient GitHubActionsWorkflowDispatchClient
}

type GitHubActionsReleaseRunnerOption func(*GitHubActionsReleaseRunner)

func NewGitHubActionsReleaseRunner(options ...GitHubActionsReleaseRunnerOption) *GitHubActionsReleaseRunner {
	client, _ := NewGitHubActionsDispatchClient()
	runner := &GitHubActionsReleaseRunner{
		coordinator:    NewGitReleaseCoordinator(),
		tokenResolver:  EnvironmentGitHubActionsDispatchTokenResolver{},
		dispatchClient: client,
	}
	for _, option := range options {
		if option != nil {
			option(runner)
		}
	}
	return runner
}

func WithGitHubActionsReleaseTokenResolver(resolver GitHubActionsDispatchTokenResolver) GitHubActionsReleaseRunnerOption {
	return func(runner *GitHubActionsReleaseRunner) {
		if resolver != nil {
			runner.tokenResolver = resolver
		}
	}
}

func WithGitHubActionsReleaseDispatchClient(client GitHubActionsWorkflowDispatchClient) GitHubActionsReleaseRunnerOption {
	return func(runner *GitHubActionsReleaseRunner) {
		if client != nil {
			runner.dispatchClient = client
		}
	}
}

// GitHubActionsReleaseResult is safe user-facing release execution metadata.
type GitHubActionsReleaseResult struct {
	Unit                 string
	Version              string
	Tag                  string
	CommitSHA            string
	Workflow             string
	ExecutionJournalPath string
	DispatchJournalPath  string
	ExecutionState       ReleaseExecutionJournalState
	DispatchState        DispatchJournalState
	RecoveryGuidance     string
	DispatchRunURL       string
}

func (runner *GitHubActionsReleaseRunner) Run(ctx context.Context, execCtx *ReleaseExecutionContext) (*GitHubActionsReleaseResult, error) {
	if execCtx == nil {
		return nil, fmt.Errorf("release execution context is missing")
	}
	if execCtx.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, fmt.Errorf("github-actions release runner supports V2 repositories only")
	}
	if execCtx.Delivery != string(releaseconfig.DeliveryGitHubActions) {
		return nil, fmt.Errorf("V2 local release execution is not available yet")
	}
	if strings.TrimSpace(execCtx.Workflow) == "" {
		return nil, fmt.Errorf("github-actions release requires a validated workflow")
	}
	token, err := runner.tokenResolver.ResolveGitHubActionsDispatchToken(ctx)
	if err != nil {
		return nil, err
	}
	materializer, err := ResolveVersionMaterializer(execCtx.Executor)
	if err != nil {
		return nil, err
	}
	materializationPlan, err := materializer.Plan(execCtx)
	if err != nil {
		return nil, err
	}
	if validationErr := materializer.Validate(materializationPlan); validationErr != nil {
		return nil, validationErr
	}
	knownFiles, err := NewKnownReleaseFiles(execCtx, materializationPlan)
	if err != nil {
		return nil, err
	}
	preflight, err := runner.coordinator.Preflight(execCtx, knownFiles)
	if err != nil {
		return nil, err
	}
	remoteURL, err := runner.coordinator.gitOutput(execCtx.RepositoryRoot, "remote", "get-url", preflight.Remote)
	if err != nil {
		return nil, fmt.Errorf("resolve V2 release remote %q: %w", preflight.Remote, err)
	}
	remoteURL = strings.TrimSpace(remoteURL)
	if _, targetErr := ResolveGitHubRepositoryTarget(preflight.Remote, remoteURL); targetErr != nil {
		return nil, targetErr
	}
	unresolved, err := NewReleaseExecutionJournalStore(execCtx.RepositoryRoot).FindUnresolved(remoteURL, execCtx.Unit.ID)
	if err != nil {
		return nil, err
	}
	if len(unresolved) > 0 {
		return nil, fmt.Errorf("unresolved V2 release execution journal exists for unit %q; use neko release resume --unit %s", execCtx.Unit.ID, execCtx.Unit.ID)
	}
	baseSHA, err := runner.coordinator.headCommit(execCtx.RepositoryRoot)
	if err != nil {
		return nil, err
	}
	journal, err := BuildReleaseExecutionJournal(execCtx, BuildReleasePlan(execCtx), knownFiles, baseSHA, remoteURL)
	if err != nil {
		return nil, err
	}
	store := NewReleaseExecutionJournalStore(execCtx.RepositoryRoot)
	resolution, err := store.Prepare(journal)
	if err != nil {
		return nil, err
	}
	journal = resolution.Journal
	if _, phaseErr := store.ConfirmPhase(journal.Identity, ReleaseExecutionPreflightValidated, ReleaseExecutionJournalUpdate{}); phaseErr != nil {
		return nil, phaseErr
	}
	materialization := NewMaterializationTransaction(materializationPlan)
	state := NewStateTransaction(execCtx.RepositoryRoot)
	commitStarted := false
	if snapshotErr := materialization.CaptureSnapshots(); snapshotErr != nil {
		_, _ = store.RecordLastError(journal.Identity, snapshotErr.Error())
		return nil, snapshotErr
	}
	if materializationErr := storeAndRun(store, journal.Identity, ReleaseExecutionPendingApplyMaterialization, func() error {
		_, applyErr := materialization.Apply()
		return applyErr
	}, ReleaseExecutionMaterializationApplied, ReleaseExecutionJournalUpdate{}); materializationErr != nil {
		_, _ = store.RecordLastError(journal.Identity, materializationErr.Error())
		return nil, materializationErr
	}
	if stateSnapshotErr := state.CaptureSnapshot(); stateSnapshotErr != nil {
		_ = materialization.Restore()
		_, _ = store.RecordLastError(journal.Identity, stateSnapshotErr.Error())
		return nil, stateSnapshotErr
	}
	if stateErr := storeAndRun(store, journal.Identity, ReleaseExecutionPendingWriteState, func() error {
		return state.WriteUnitVersion(execCtx.Unit.ID, execCtx.NextVersion)
	}, ReleaseExecutionStateWritten, ReleaseExecutionJournalUpdate{}); stateErr != nil {
		_ = materialization.Restore()
		_, _ = store.RecordLastError(journal.Identity, stateErr.Error())
		return nil, stateErr
	}
	if stageErr := storeAndRun(store, journal.Identity, ReleaseExecutionPendingStageReleaseFiles, func() error {
		return runner.coordinator.Stage(execCtx, knownFiles)
	}, ReleaseExecutionReleaseFilesStaged, ReleaseExecutionJournalUpdate{}); stageErr != nil {
		_ = state.RestoreSnapshot()
		_ = materialization.Restore()
		_ = runner.coordinator.UnstageKnown(knownFiles)
		_, _ = store.RecordLastError(journal.Identity, stageErr.Error())
		return nil, stageErr
	}
	var commitSHA string
	if _, pendingErr := store.BeginPending(journal.Identity, ReleaseExecutionPendingCreateReleaseCommit); pendingErr != nil {
		return nil, pendingErr
	}
	commitStarted = true
	commitSHA, err = runner.coordinator.Commit(execCtx, knownFiles)
	if err == nil {
		_, err = store.ConfirmPhase(journal.Identity, ReleaseExecutionCommitCreated, ReleaseExecutionJournalUpdate{ReleaseCommitSHA: commitSHA})
	}
	if err != nil {
		if !commitStarted {
			_ = state.RestoreSnapshot()
			_ = materialization.Restore()
			_ = runner.coordinator.UnstageKnown(knownFiles)
		}
		_, _ = store.RecordLastError(journal.Identity, err.Error())
		return nil, err
	}
	if _, tagPendingErr := store.BeginPending(journal.Identity, ReleaseExecutionPendingCreateUnitTag); tagPendingErr != nil {
		return nil, tagPendingErr
	}
	if _, tagErr := runner.coordinator.CreateTag(execCtx, commitSHA); tagErr != nil {
		_, _ = store.RecordLastError(journal.Identity, tagErr.Error())
		return nil, tagErr
	}
	if _, tagPhaseErr := store.ConfirmPhase(journal.Identity, ReleaseExecutionTagCreated, ReleaseExecutionJournalUpdate{TagTargetSHA: commitSHA}); tagPhaseErr != nil {
		return nil, tagPhaseErr
	}
	gitResult := &GitReleaseResult{
		Unit:                 execCtx.Unit.ID,
		Version:              execCtx.NextVersion,
		Tag:                  execCtx.Tag,
		CommitSHA:            commitSHA,
		RepositoryRemoteName: preflight.Remote,
		RepositoryRemote:     remoteURL,
		CommitCreated:        true,
		TagCreated:           true,
		ReachedPhase:         "tag-created",
		KnownReleaseFiles:    knownFiles.RelativePaths(),
	}
	dispatchRequest, err := BuildReleaseDispatchRequest(execCtx, gitResult)
	if err != nil {
		_, _ = store.RecordLastError(journal.Identity, err.Error())
		return nil, err
	}
	dispatchStore := NewDispatchJournalStore(execCtx.RepositoryRoot)
	if _, dispatchPendingErr := store.BeginPending(journal.Identity, ReleaseExecutionPendingCreateDispatchJournal); dispatchPendingErr != nil {
		return nil, dispatchPendingErr
	}
	dispatchResolution, err := dispatchStore.Prepare(dispatchRequest)
	if err != nil {
		_, _ = store.RecordLastError(journal.Identity, err.Error())
		return nil, err
	}
	if _, dispatchPhaseErr := store.ConfirmPhase(journal.Identity, ReleaseExecutionDispatchJournalPrepared, ReleaseExecutionJournalUpdate{DispatchJournalIdentity: dispatchRequest.Identity.SHA256}); dispatchPhaseErr != nil {
		return nil, dispatchPhaseErr
	}
	if commitPushErr := storeAndRun(store, journal.Identity, ReleaseExecutionPendingPushReleaseCommit, func() error {
		return runner.coordinator.PushCommit(execCtx, preflight.Remote, preflight.UpstreamBranch, commitSHA)
	}, ReleaseExecutionCommitPushed, ReleaseExecutionJournalUpdate{CommitPushStatus: "pushed"}); commitPushErr != nil {
		_, _ = store.RecordLastError(journal.Identity, commitPushErr.Error())
		return nil, commitPushErr
	}
	gitResult.CommitPushed = true
	if tagPushErr := storeAndRun(store, journal.Identity, ReleaseExecutionPendingPushUnitTag, func() error {
		return runner.coordinator.PushTag(execCtx, preflight.Remote, execCtx.Tag, commitSHA)
	}, ReleaseExecutionTagPushed, ReleaseExecutionJournalUpdate{TagPushStatus: "pushed"}); tagPushErr != nil {
		_, _ = store.RecordLastError(journal.Identity, tagPushErr.Error())
		return nil, tagPushErr
	}
	gitResult.TagPushed = true
	dispatcher, err := NewGitHubActionsDispatcher(execCtx.RepositoryRoot,
		WithGitHubActionsDispatcherClient(runner.dispatchClient),
		WithGitHubActionsDispatcherTokenResolver(staticGitHubActionsDispatchTokenResolver{token: token}),
	)
	if err != nil {
		return nil, err
	}
	dispatchResult, err := dispatcher.Dispatch(ctx, dispatchRequest)
	if err != nil {
		_, _ = store.RecordLastError(journal.Identity, err.Error())
		return nil, err
	}
	if !dispatchResult.Accepted {
		_, _ = store.RecordLastError(journal.Identity, dispatchResult.RecoveryGuidance)
		return &GitHubActionsReleaseResult{
			Unit:                 execCtx.Unit.ID,
			Version:              execCtx.NextVersion,
			Tag:                  execCtx.Tag,
			CommitSHA:            commitSHA,
			Workflow:             execCtx.Workflow,
			ExecutionJournalPath: resolution.Path,
			DispatchJournalPath:  dispatchResolution.Path,
			ExecutionState:       ReleaseExecutionTagPushed,
			DispatchState:        dispatchResult.State,
			RecoveryGuidance:     dispatchResult.RecoveryGuidance,
			DispatchRunURL:       dispatchResult.HTMLURL,
		}, fmt.Errorf("GitHub Actions dispatch was not accepted: %s", dispatchResult.RecoveryGuidance)
	}
	if _, err := store.ConfirmPhase(journal.Identity, ReleaseExecutionHandoffReady, ReleaseExecutionJournalUpdate{}); err != nil {
		return nil, err
	}
	return &GitHubActionsReleaseResult{
		Unit:                 execCtx.Unit.ID,
		Version:              execCtx.NextVersion,
		Tag:                  execCtx.Tag,
		CommitSHA:            commitSHA,
		Workflow:             execCtx.Workflow,
		ExecutionJournalPath: resolution.Path,
		DispatchJournalPath:  dispatchResult.JournalPath,
		ExecutionState:       ReleaseExecutionHandoffReady,
		DispatchState:        dispatchResult.State,
		RecoveryGuidance:     "GitHub Actions dispatch accepted. GitHub Actions owns build and publish from the pushed tag.",
		DispatchRunURL:       dispatchResult.HTMLURL,
	}, nil
}

func storeAndRun(store *ReleaseExecutionJournalStore, identity ReleaseExecutionIdentity, action ReleaseExecutionPendingAction, mutation func() error, phase ReleaseExecutionJournalState, update ReleaseExecutionJournalUpdate) error {
	if _, err := store.BeginPending(identity, action); err != nil {
		return err
	}
	if err := mutation(); err != nil {
		return err
	}
	if _, err := store.ConfirmPhase(identity, phase, update); err != nil {
		return err
	}
	return nil
}

type staticGitHubActionsDispatchTokenResolver struct {
	token string
}

func (resolver staticGitHubActionsDispatchTokenResolver) ResolveGitHubActionsDispatchToken(_ context.Context) (string, error) {
	if strings.TrimSpace(resolver.token) == "" {
		return "", missingGitHubActionsDispatchTokenError()
	}
	return resolver.token, nil
}

func githubActionsReleaseResponse(command string, result *GitHubActionsReleaseResult) *plugin.Response {
	items := []map[string]any{
		{"property": "Unit", "value": result.Unit},
		{"property": "Version", "value": result.Version},
		{"property": "Tag", "value": result.Tag},
		{"property": "Release Commit", "value": result.CommitSHA},
		{"property": "Workflow", "value": result.Workflow},
		{"property": "Execution Journal", "value": result.ExecutionJournalPath},
		{"property": "Dispatch Journal", "value": result.DispatchJournalPath},
		{"property": "Execution State", "value": string(result.ExecutionState)},
		{"property": "Dispatch State", "value": string(result.DispatchState)},
		{"property": "Dispatch Run", "value": emptyFallback(result.DispatchRunURL, "not reported")},
		{"property": "Status", "value": result.RecoveryGuidance},
	}
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   command,
			Timestamp: time.Now(),
		},
		Data:         map[string]any{"items": items},
		RendererHint: "table",
	}
}

func emptyFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
