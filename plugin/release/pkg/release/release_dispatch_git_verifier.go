package release

import (
	"encoding/json"
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type releaseDispatchGitVerifier interface {
	VerifyRelease(ctx *ReleaseExecutionContext, result *GitReleaseResult) error
}

// gitReleaseDispatchVerifier owns the two read-only Git facts required before a
// dispatch request can be built. It does not create requests or mutate Git.
type gitReleaseDispatchVerifier struct {
	coordinator *GitReleaseCoordinator
}

func (verifier gitReleaseDispatchVerifier) VerifyRelease(ctx *ReleaseExecutionContext, result *GitReleaseResult) error {
	tagCommit, err := verifier.coordinator.tagCommit(ctx.RepositoryRoot, ctx.Tag)
	if err != nil {
		return err
	}
	if tagCommit != result.CommitSHA {
		return fmt.Errorf("unit tag %q points to %s, expected release commit %s", ctx.Tag, tagCommit, result.CommitSHA)
	}
	committedVersion, err := verifier.committedUnitVersion(ctx.RepositoryRoot, result.CommitSHA, ctx.Unit.ID)
	if err != nil {
		return err
	}
	if committedVersion != ctx.NextVersion {
		return fmt.Errorf("committed V2 state unit %q version = %q, expected %q", ctx.Unit.ID, committedVersion, ctx.NextVersion)
	}
	return nil
}

func (verifier gitReleaseDispatchVerifier) committedUnitVersion(repositoryRoot, commitSHA, unitID string) (string, error) {
	stateContent, err := verifier.coordinator.gitOutput(repositoryRoot, "show", commitSHA+":"+releaseconfig.V2Directory+"/"+releaseconfig.V2StateFileName)
	if err != nil {
		return "", fmt.Errorf("inspect V2 state in release commit %s: %w", commitSHA, err)
	}
	var state releaseconfig.V2ReleaseState
	if err := json.Unmarshal([]byte(stateContent), &state); err != nil {
		return "", fmt.Errorf("decode V2 state in release commit %s: %w", commitSHA, err)
	}
	unitState, ok := state.Units[unitID]
	if !ok {
		return "", fmt.Errorf("release commit %s state is missing unit %q", commitSHA, unitID)
	}
	return unitState.Version, nil
}
