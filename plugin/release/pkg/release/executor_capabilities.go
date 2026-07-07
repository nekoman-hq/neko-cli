package release

import (
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
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
	switch releaseconfig.ExecutorType(executorType) {
	case releaseconfig.ExecutorGoReleaser:
		return ExecutorCapabilities{
			Type:                          string(releaseconfig.ExecutorGoReleaser),
			UpdatesVersionFiles:           false,
			CreatesCommit:                 true,
			CreatesTag:                    true,
			Pushes:                        true,
			CreatesGitHubRelease:          true,
			SupportsLocalExecution:        true,
			SupportsDryRun:                true,
			RequiresRepositoryCleanliness: true,
			MayRequireRollback:            true,
			VersionFilesOwner:             "none",
			CommitOwner:                   "neko-cli",
			TagOwner:                      "neko-cli",
			PushOwner:                     "neko-cli",
			GitHubReleaseOwner:            "goreleaser",
			StateBeforeExecutor:           true,
			StateCommitGuaranteed:         true,
			StateCommitGuarantee:          "Neko CLI writes and stages .neko/release.state.json before its release commit",
		}, nil
	case releaseconfig.ExecutorJReleaser:
		return ExecutorCapabilities{
			Type:                          string(releaseconfig.ExecutorJReleaser),
			UpdatesVersionFiles:           true,
			CreatesCommit:                 true,
			CreatesTag:                    true,
			Pushes:                        true,
			CreatesGitHubRelease:          true,
			SupportsLocalExecution:        true,
			SupportsDryRun:                true,
			RequiresRepositoryCleanliness: true,
			MayRequireRollback:            true,
			VersionFilesOwner:             "neko-cli",
			CommitOwner:                   "neko-cli",
			TagOwner:                      "jreleaser",
			PushOwner:                     "neko-cli + jreleaser",
			GitHubReleaseOwner:            "jreleaser",
			StateBeforeExecutor:           true,
			StateCommitGuaranteed:         true,
			StateCommitGuarantee:          "Neko CLI writes and stages .neko/release.state.json before its release metadata commit; JReleaser creates the tag and release later",
		}, nil
	case releaseconfig.ExecutorReleaseIt:
		return ExecutorCapabilities{
			Type:                          string(releaseconfig.ExecutorReleaseIt),
			UpdatesVersionFiles:           true,
			CreatesCommit:                 true,
			CreatesTag:                    true,
			Pushes:                        true,
			CreatesGitHubRelease:          true,
			SupportsLocalExecution:        true,
			SupportsDryRun:                false,
			RequiresRepositoryCleanliness: true,
			MayRequireRollback:            true,
			VersionFilesOwner:             "release-it",
			CommitOwner:                   "release-it",
			TagOwner:                      "release-it",
			PushOwner:                     "release-it",
			GitHubReleaseOwner:            "release-it",
			StateBeforeExecutor:           true,
			StateCommitGuaranteed:         false,
			StateCommitGuarantee:          "release-it owns commit, tag, push, and GitHub release creation; root V2 state cannot be guaranteed in that commit from a nested unit root",
			V2LocalExecutionBlockedReason: "V2 local release-it is blocked because .neko/release.state.json lives at the repository root and release-it owns the release commit from the unit root",
		}, nil
	default:
		return ExecutorCapabilities{}, fmt.Errorf("unknown executor: %s", executorType)
	}
}
