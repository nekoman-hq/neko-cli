package release

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const releaseExecutionJournalSchemaVersion = 1

// ReleaseExecutionJournal records the durable boundary of one V2 local release
// transaction. It is separate from DispatchJournal, which records one later
// GitHub Actions HTTP dispatch attempt.
//
//nolint:govet // Persisted fields follow release lifecycle order.
type ReleaseExecutionJournal struct {
	SchemaVersion           int                            `json:"schemaVersion"`
	Identity                ReleaseExecutionIdentity       `json:"identity"`
	RepositoryRemote        string                         `json:"repositoryRemote"`
	RepositoryRootHint      string                         `json:"repositoryRootHint,omitempty"`
	BaseCommitSHA           string                         `json:"baseCommitSHA"`
	UnitID                  string                         `json:"unit"`
	CurrentVersion          string                         `json:"currentVersion"`
	NextVersion             string                         `json:"nextVersion"`
	Tag                     string                         `json:"tag"`
	Executor                string                         `json:"executor"`
	Delivery                string                         `json:"delivery"`
	WorkflowPath            string                         `json:"workflowPath,omitempty"`
	KnownReleaseFiles       []ReleaseExecutionFileMetadata `json:"knownReleaseFiles"`
	State                   ReleaseExecutionJournalState   `json:"state"`
	PendingAction           ReleaseExecutionPendingAction  `json:"pendingAction"`
	ReleaseCommitSHA        string                         `json:"releaseCommitSHA,omitempty"`
	TagTargetSHA            string                         `json:"tagTargetSHA,omitempty"`
	DispatchJournalIdentity string                         `json:"dispatchJournalIdentity,omitempty"`
	CommitPushStatus        string                         `json:"commitPushStatus,omitempty"`
	TagPushStatus           string                         `json:"tagPushStatus,omitempty"`
	CreatedAt               time.Time                      `json:"createdAt"`
	UpdatedAt               time.Time                      `json:"updatedAt"`
	LastError               string                         `json:"lastError,omitempty"`
	RecoveryMetadata        ReleaseExecutionRecoveryMeta   `json:"recoveryMetadata"`
}

// ReleaseExecutionFileMetadata stores hashes and relative paths only. Absolute
// paths are useful while assessing a local checkout, but are intentionally not
// persisted in the journal.
//
//nolint:govet // File fields are ordered by recovery meaning.
type ReleaseExecutionFileMetadata struct {
	AbsolutePath             string `json:"-"`
	RepositoryRelativePath   string `json:"repositoryRelativePath"`
	ExpectedExistsBefore     bool   `json:"expectedExistsBefore"`
	ExpectedExistsAfter      bool   `json:"expectedExistsAfter"`
	PreimageSHA256           string `json:"preimageSHA256,omitempty"`
	PostimageSHA256          string `json:"postimageSHA256,omitempty"`
	RequiredForReleaseCommit bool   `json:"requiredForReleaseCommit"`
	Reason                   string `json:"reason"`
}

// ReleaseExecutionRecoveryMeta stores safe local evidence from recovery
// assessment. It never stores secrets or file contents.
type ReleaseExecutionRecoveryMeta struct {
	LastAssessmentAt time.Time `json:"lastAssessmentAt,omitempty"`
	LastStatus       string    `json:"lastStatus,omitempty"`
}

type ReleaseExecutionJournalState string

const (
	ReleaseExecutionPrepared                ReleaseExecutionJournalState = "prepared"
	ReleaseExecutionPreflightValidated      ReleaseExecutionJournalState = "preflight-validated"
	ReleaseExecutionMaterializationApplied  ReleaseExecutionJournalState = "materialization-applied"
	ReleaseExecutionStateWritten            ReleaseExecutionJournalState = "state-written"
	ReleaseExecutionReleaseFilesStaged      ReleaseExecutionJournalState = "release-files-staged"
	ReleaseExecutionCommitCreated           ReleaseExecutionJournalState = "commit-created"
	ReleaseExecutionTagCreated              ReleaseExecutionJournalState = "tag-created"
	ReleaseExecutionDispatchJournalPrepared ReleaseExecutionJournalState = "dispatch-journal-prepared"
	ReleaseExecutionCommitPushed            ReleaseExecutionJournalState = "commit-pushed"
	ReleaseExecutionTagPushed               ReleaseExecutionJournalState = "tag-pushed"
	ReleaseExecutionHandoffReady            ReleaseExecutionJournalState = "handoff-ready"
)

type ReleaseExecutionPendingAction string

const (
	ReleaseExecutionPendingNone                  ReleaseExecutionPendingAction = "none"
	ReleaseExecutionPendingApplyMaterialization  ReleaseExecutionPendingAction = "apply-materialization"
	ReleaseExecutionPendingWriteState            ReleaseExecutionPendingAction = "write-state"
	ReleaseExecutionPendingStageReleaseFiles     ReleaseExecutionPendingAction = "stage-release-files"
	ReleaseExecutionPendingCreateReleaseCommit   ReleaseExecutionPendingAction = "create-release-commit"
	ReleaseExecutionPendingCreateUnitTag         ReleaseExecutionPendingAction = "create-unit-tag"
	ReleaseExecutionPendingCreateDispatchJournal ReleaseExecutionPendingAction = "create-dispatch-journal"
	ReleaseExecutionPendingPushReleaseCommit     ReleaseExecutionPendingAction = "push-release-commit"
	ReleaseExecutionPendingPushUnitTag           ReleaseExecutionPendingAction = "push-unit-tag"
)

// ReleaseExecutionJournalUpdate contains once-only metadata attached to a
// confirmed phase transition.
type ReleaseExecutionJournalUpdate struct {
	ReleaseCommitSHA        string
	TagTargetSHA            string
	DispatchJournalIdentity string
	CommitPushStatus        string
	TagPushStatus           string
	LastError               string
}

// BuildReleaseExecutionJournal builds a deterministic, non-mutating journal for
// an intended V2 release transaction.
func BuildReleaseExecutionJournal(ctx *ReleaseExecutionContext, plan ReleasePlan, files KnownReleaseFiles, baseCommitSHA, repositoryRemote string) (*ReleaseExecutionJournal, error) {
	if ctx == nil {
		return nil, fmt.Errorf("release execution context is missing")
	}
	if ctx.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, fmt.Errorf("release execution journals support V2 repositories only")
	}
	if strings.TrimSpace(repositoryRemote) == "" {
		return nil, fmt.Errorf("release execution journal requires repository remote identity")
	}
	if !fullGitSHARegexp.MatchString(strings.TrimSpace(baseCommitSHA)) {
		return nil, fmt.Errorf("release execution journal requires full base commit SHA, got %q", baseCommitSHA)
	}
	if !ctx.TagSpec.Matches(ctx.Tag) {
		return nil, fmt.Errorf("target tag %q does not match unit %q tag prefix %q", ctx.Tag, ctx.Unit.ID, ctx.TagSpec.Prefix)
	}
	if parsedVersion, ok := ctx.TagSpec.Parse(ctx.Tag); !ok || parsedVersion != ctx.NextVersion {
		return nil, fmt.Errorf("target tag %q does not encode next version %q", ctx.Tag, ctx.NextVersion)
	}
	if plan.UnitID != "" && plan.UnitID != ctx.Unit.ID {
		return nil, fmt.Errorf("release plan unit %q does not match context unit %q", plan.UnitID, ctx.Unit.ID)
	}
	if ctx.Delivery == string(releaseconfig.DeliveryGitHubActions) && strings.TrimSpace(ctx.Workflow) == "" {
		return nil, fmt.Errorf("github-actions delivery requires a workflow path")
	}
	if ctx.Delivery == string(releaseconfig.DeliveryLocal) && strings.TrimSpace(ctx.Workflow) != "" {
		return nil, fmt.Errorf("local delivery must not carry a workflow path")
	}
	if files.RepositoryRoot == "" {
		files.RepositoryRoot = ctx.RepositoryRoot
	}
	if err := files.Validate(); err != nil {
		return nil, err
	}
	identity, err := newReleaseExecutionIdentity(repositoryRemote, baseCommitSHA, ctx.Unit.ID, ctx.CurrentVersion, ctx.NextVersion, ctx.Tag, ctx.Executor, ctx.Delivery, ctx.Workflow)
	if err != nil {
		return nil, err
	}
	metadata, err := buildReleaseExecutionFileMetadata(ctx.RepositoryRoot, files)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	journal := &ReleaseExecutionJournal{
		SchemaVersion:      releaseExecutionJournalSchemaVersion,
		Identity:           identity,
		RepositoryRemote:   strings.TrimSpace(repositoryRemote),
		RepositoryRootHint: ctx.RepositoryRoot,
		BaseCommitSHA:      strings.TrimSpace(baseCommitSHA),
		UnitID:             ctx.Unit.ID,
		CurrentVersion:     ctx.CurrentVersion,
		NextVersion:        ctx.NextVersion,
		Tag:                ctx.Tag,
		Executor:           ctx.Executor,
		Delivery:           ctx.Delivery,
		WorkflowPath:       ctx.Workflow,
		KnownReleaseFiles:  metadata,
		State:              ReleaseExecutionPrepared,
		PendingAction:      ReleaseExecutionPendingNone,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := journal.ValidateImmutable(journal); err != nil {
		return nil, err
	}
	return journal, nil
}

func buildReleaseExecutionFileMetadata(repositoryRoot string, files KnownReleaseFiles) ([]ReleaseExecutionFileMetadata, error) {
	metadata := make([]ReleaseExecutionFileMetadata, 0, len(files.Files))
	for _, file := range files.Files {
		normalized, err := newKnownReleaseFile(repositoryRoot, file.AbsolutePath)
		if err != nil {
			return nil, err
		}
		entry := ReleaseExecutionFileMetadata{
			AbsolutePath:             normalized.AbsolutePath,
			RepositoryRelativePath:   normalized.RepositoryRelativePath,
			ExpectedExistsBefore:     file.ExpectedExistsBefore,
			ExpectedExistsAfter:      file.ExpectedExistsAfter,
			PreimageSHA256:           file.PreimageSHA256,
			PostimageSHA256:          file.PostimageSHA256,
			RequiredForReleaseCommit: file.RequiredForReleaseCommit,
			Reason:                   file.Reason,
		}
		if entry.Reason == "" {
			entry.Reason = releaseExecutionFileReason(normalized.RepositoryRelativePath)
		}
		if !entry.ExpectedExistsBefore && entry.PreimageSHA256 == "" {
			hash, existed, err := hashFileIfExists(normalized.AbsolutePath)
			if err != nil {
				return nil, err
			}
			entry.ExpectedExistsBefore = existed
			entry.PreimageSHA256 = hash
		}
		if !entry.ExpectedExistsAfter {
			entry.ExpectedExistsAfter = true
		}
		if !entry.RequiredForReleaseCommit {
			entry.RequiredForReleaseCommit = true
		}
		metadata = append(metadata, entry)
	}
	return metadata, nil
}

func releaseExecutionFileReason(relativePath string) string {
	if relativePath == filepath.ToSlash(filepath.Join(releaseconfig.V2Directory, releaseconfig.V2StateFileName)) {
		return "v2 release state"
	}
	return "version materialization"
}

func (journal *ReleaseExecutionJournal) ValidateImmutable(expected *ReleaseExecutionJournal) error {
	if journal == nil || expected == nil {
		return fmt.Errorf("release execution journal is missing")
	}
	if journal.SchemaVersion != releaseExecutionJournalSchemaVersion {
		return fmt.Errorf("release execution journal schemaVersion mismatch")
	}
	if !journal.State.Valid() {
		return fmt.Errorf("release execution journal state %q is invalid", journal.State)
	}
	if !journal.PendingAction.Valid() {
		return fmt.Errorf("release execution pending action %q is invalid", journal.PendingAction)
	}
	if journal.Identity.SHA256 != expected.Identity.SHA256 ||
		journal.RepositoryRemote != expected.RepositoryRemote ||
		journal.BaseCommitSHA != expected.BaseCommitSHA ||
		journal.UnitID != expected.UnitID ||
		journal.CurrentVersion != expected.CurrentVersion ||
		journal.NextVersion != expected.NextVersion ||
		journal.Tag != expected.Tag ||
		journal.Executor != expected.Executor ||
		journal.Delivery != expected.Delivery ||
		journal.WorkflowPath != expected.WorkflowPath {
		return fmt.Errorf("release execution journal immutable fields do not match")
	}
	if !sameReleaseExecutionFiles(journal.KnownReleaseFiles, expected.KnownReleaseFiles) {
		return fmt.Errorf("release execution journal known release files do not match")
	}
	return nil
}

func (journal *ReleaseExecutionJournal) BeginPending(action ReleaseExecutionPendingAction, now time.Time) error {
	if action == ReleaseExecutionPendingNone || !action.Valid() {
		return fmt.Errorf("release execution pending action %q is invalid", action)
	}
	if journal.PendingAction != ReleaseExecutionPendingNone {
		return fmt.Errorf("release execution pending action %s is already set", journal.PendingAction)
	}
	journal.PendingAction = action
	journal.touch(now)
	return nil
}

func (journal *ReleaseExecutionJournal) ConfirmPhase(next ReleaseExecutionJournalState, update ReleaseExecutionJournalUpdate, now time.Time) error {
	if err := validateReleaseExecutionTransition(journal.State, next); err != nil {
		return err
	}
	expectedPending := pendingActionForConfirmedPhase(next)
	if expectedPending != ReleaseExecutionPendingNone && journal.PendingAction != expectedPending {
		return fmt.Errorf("release execution phase %s requires pending action %s, got %s", next, expectedPending, journal.PendingAction)
	}
	if expectedPending == ReleaseExecutionPendingNone && journal.PendingAction != ReleaseExecutionPendingNone {
		return fmt.Errorf("release execution pending action %s must be cleared before confirming %s", journal.PendingAction, next)
	}
	if err := journal.applyOnceOnlyUpdate(next, update); err != nil {
		return err
	}
	journal.State = next
	journal.PendingAction = ReleaseExecutionPendingNone
	journal.LastError = update.LastError
	journal.touch(now)
	return nil
}

func (journal *ReleaseExecutionJournal) touch(now time.Time) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	journal.UpdatedAt = now.UTC()
}

func (journal *ReleaseExecutionJournal) applyOnceOnlyUpdate(next ReleaseExecutionJournalState, update ReleaseExecutionJournalUpdate) error {
	if update.ReleaseCommitSHA != "" {
		if !fullGitSHARegexp.MatchString(update.ReleaseCommitSHA) {
			return fmt.Errorf("release commit SHA %q is invalid", update.ReleaseCommitSHA)
		}
		if journal.ReleaseCommitSHA != "" && journal.ReleaseCommitSHA != update.ReleaseCommitSHA {
			return fmt.Errorf("release commit SHA is already set")
		}
		journal.ReleaseCommitSHA = update.ReleaseCommitSHA
	}
	if update.TagTargetSHA != "" {
		if !fullGitSHARegexp.MatchString(update.TagTargetSHA) {
			return fmt.Errorf("tag target SHA %q is invalid", update.TagTargetSHA)
		}
		if journal.TagTargetSHA != "" && journal.TagTargetSHA != update.TagTargetSHA {
			return fmt.Errorf("tag target SHA is already set")
		}
		journal.TagTargetSHA = update.TagTargetSHA
	}
	if update.DispatchJournalIdentity != "" {
		if !isSafeDispatchIdentityHash(update.DispatchJournalIdentity) {
			return fmt.Errorf("dispatch journal identity %q is invalid", update.DispatchJournalIdentity)
		}
		if journal.DispatchJournalIdentity != "" && journal.DispatchJournalIdentity != update.DispatchJournalIdentity {
			return fmt.Errorf("dispatch journal identity is already set")
		}
		journal.DispatchJournalIdentity = update.DispatchJournalIdentity
	}
	if update.CommitPushStatus != "" {
		if next != ReleaseExecutionCommitPushed {
			return fmt.Errorf("commit push status can be set only at commit-pushed")
		}
		if journal.CommitPushStatus != "" && journal.CommitPushStatus != update.CommitPushStatus {
			return fmt.Errorf("commit push status is already set")
		}
		journal.CommitPushStatus = update.CommitPushStatus
	}
	if update.TagPushStatus != "" {
		if next != ReleaseExecutionTagPushed {
			return fmt.Errorf("tag push status can be set only at tag-pushed")
		}
		if journal.TagPushStatus != "" && journal.TagPushStatus != update.TagPushStatus {
			return fmt.Errorf("tag push status is already set")
		}
		journal.TagPushStatus = update.TagPushStatus
	}
	return nil
}

func (state ReleaseExecutionJournalState) Valid() bool {
	return releaseExecutionStateRank(state) >= 0
}

func (state ReleaseExecutionJournalState) CanTransitionTo(next ReleaseExecutionJournalState) bool {
	currentRank := releaseExecutionStateRank(state)
	nextRank := releaseExecutionStateRank(next)
	return currentRank >= 0 && nextRank == currentRank+1
}

func validateReleaseExecutionTransition(current, next ReleaseExecutionJournalState) error {
	if !current.Valid() {
		return fmt.Errorf("release execution state %q is invalid", current)
	}
	if !next.Valid() {
		return fmt.Errorf("release execution target state %q is invalid", next)
	}
	if !current.CanTransitionTo(next) {
		return fmt.Errorf("release execution transition %s -> %s is invalid", current, next)
	}
	return nil
}

func releaseExecutionStateRank(state ReleaseExecutionJournalState) int {
	for index, candidate := range releaseExecutionStateOrder {
		if state == candidate {
			return index
		}
	}
	return -1
}

var releaseExecutionStateOrder = []ReleaseExecutionJournalState{
	ReleaseExecutionPrepared,
	ReleaseExecutionPreflightValidated,
	ReleaseExecutionMaterializationApplied,
	ReleaseExecutionStateWritten,
	ReleaseExecutionReleaseFilesStaged,
	ReleaseExecutionCommitCreated,
	ReleaseExecutionTagCreated,
	ReleaseExecutionDispatchJournalPrepared,
	ReleaseExecutionCommitPushed,
	ReleaseExecutionTagPushed,
	ReleaseExecutionHandoffReady,
}

func (action ReleaseExecutionPendingAction) Valid() bool {
	switch action {
	case ReleaseExecutionPendingNone,
		ReleaseExecutionPendingApplyMaterialization,
		ReleaseExecutionPendingWriteState,
		ReleaseExecutionPendingStageReleaseFiles,
		ReleaseExecutionPendingCreateReleaseCommit,
		ReleaseExecutionPendingCreateUnitTag,
		ReleaseExecutionPendingCreateDispatchJournal,
		ReleaseExecutionPendingPushReleaseCommit,
		ReleaseExecutionPendingPushUnitTag:
		return true
	default:
		return false
	}
}

func pendingActionForConfirmedPhase(phase ReleaseExecutionJournalState) ReleaseExecutionPendingAction {
	switch phase {
	case ReleaseExecutionMaterializationApplied:
		return ReleaseExecutionPendingApplyMaterialization
	case ReleaseExecutionStateWritten:
		return ReleaseExecutionPendingWriteState
	case ReleaseExecutionReleaseFilesStaged:
		return ReleaseExecutionPendingStageReleaseFiles
	case ReleaseExecutionCommitCreated:
		return ReleaseExecutionPendingCreateReleaseCommit
	case ReleaseExecutionTagCreated:
		return ReleaseExecutionPendingCreateUnitTag
	case ReleaseExecutionDispatchJournalPrepared:
		return ReleaseExecutionPendingCreateDispatchJournal
	case ReleaseExecutionCommitPushed:
		return ReleaseExecutionPendingPushReleaseCommit
	case ReleaseExecutionTagPushed:
		return ReleaseExecutionPendingPushUnitTag
	default:
		return ReleaseExecutionPendingNone
	}
}

func sameReleaseExecutionFiles(a, b []ReleaseExecutionFileMetadata) bool {
	if len(a) != len(b) {
		return false
	}
	encoded := func(files []ReleaseExecutionFileMetadata) (string, error) {
		clone := make([]ReleaseExecutionFileMetadata, len(files))
		copy(clone, files)
		for i := range clone {
			clone[i].AbsolutePath = ""
		}
		data, err := json.Marshal(clone)
		return string(data), err
	}
	left, leftErr := encoded(a)
	right, rightErr := encoded(b)
	return leftErr == nil && rightErr == nil && left == right
}

func hashFileIfExists(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("hash release execution file %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), true, nil
}
