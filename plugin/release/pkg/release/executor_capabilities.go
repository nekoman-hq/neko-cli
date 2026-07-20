package release

import (
	"fmt"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
)

// ExecutorCapabilities documents the ownership boundary between Neko CLI and
// the selected release executor.
//
//nolint:govet // Logical capability order is clearer than fieldalignment ordering here.
type ExecutorCapabilities struct {
	Type                          string
	UpdatesVersionFiles           bool
	CreatesCommit                 bool
	CreatesTag                    bool
	Pushes                        bool
	CreatesGitHubRelease          bool
	SupportsLocalExecution        bool
	SupportsDryRun                bool
	RequiresRepositoryCleanliness bool
	MayRequireRollback            bool
	VersionFilesOwner             string
	CommitOwner                   string
	TagOwner                      string
	PushOwner                     string
	GitHubReleaseOwner            string
	StateBeforeExecutor           bool
	StateCommitGuaranteed         bool
	StateCommitGuarantee          string
	V2LocalExecutionBlockedReason string
}

// ResolveExecutorCapabilities returns the current local-executor behavior.
func ResolveExecutorCapabilities(executorType string) (ExecutorCapabilities, error) {
	identity, err := releasetool.ParseIdentity(executorType)
	if err != nil {
		return ExecutorCapabilities{}, fmt.Errorf("unknown executor: %s", executorType)
	}
	behavior, err := releasetool.V1BehaviorFor(identity)
	if err != nil {
		return ExecutorCapabilities{}, fmt.Errorf("unknown executor: %s", executorType)
	}
	return ExecutorCapabilities{
		Type:                          string(behavior.Identity),
		UpdatesVersionFiles:           behavior.UpdatesVersionFiles,
		CreatesCommit:                 behavior.CreatesCommit,
		CreatesTag:                    behavior.CreatesTag,
		Pushes:                        behavior.Pushes,
		CreatesGitHubRelease:          behavior.CreatesGitHubRelease,
		SupportsLocalExecution:        behavior.SupportsLocalExecution,
		SupportsDryRun:                behavior.SupportsDryRun,
		RequiresRepositoryCleanliness: behavior.RequiresRepositoryCleanliness,
		MayRequireRollback:            behavior.MayRequireRollback,
		VersionFilesOwner:             behavior.VersionFilesOwner,
		CommitOwner:                   behavior.CommitOwner,
		TagOwner:                      behavior.TagOwner,
		PushOwner:                     behavior.PushOwner,
		GitHubReleaseOwner:            behavior.GitHubReleaseOwner,
		StateBeforeExecutor:           behavior.StateBeforeExecutor,
		StateCommitGuaranteed:         behavior.StateCommitGuaranteed,
		StateCommitGuarantee:          behavior.StateCommitGuarantee,
		V2LocalExecutionBlockedReason: behavior.V2LocalExecutionBlockedReason,
	}, nil
}
