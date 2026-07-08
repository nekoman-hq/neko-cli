package release

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

var fullGitSHARegexp = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ReleaseDispatchRequest is the immutable local contract for a future GitHub
// Actions workflow dispatch. Only deterministic release facts are included as
// inputs; executor, delivery, workflow and repository paths must be read from
// checked-out repository configuration by the workflow itself.
//
//nolint:govet // Dispatch fields follow the external contract order.
type ReleaseDispatchRequest struct {
	RepositoryRemoteName string                  `json:"repositoryRemoteName"`
	UnitID               string                  `json:"unit"`
	Version              string                  `json:"version"`
	Tag                  string                  `json:"tag"`
	ReleaseCommitSHA     string                  `json:"releaseCommitSHA"`
	WorkflowPath         string                  `json:"workflowPath"`
	WorkflowFileName     string                  `json:"workflowFileName"`
	Delivery             string                  `json:"delivery"`
	Executor             string                  `json:"executor"`
	Inputs               map[string]string       `json:"inputs"`
	Identity             ReleaseDispatchIdentity `json:"identity"`
}

// BuildReleaseDispatchRequest builds a deterministic future dispatch request
// from already validated V2 release context and already completed Git release
// coordination. It performs only read-only Git inspection.
func BuildReleaseDispatchRequest(ctx *ReleaseExecutionContext, result *GitReleaseResult) (*ReleaseDispatchRequest, error) {
	if ctx == nil {
		return nil, fmt.Errorf("release execution context is missing")
	}
	if result == nil {
		return nil, fmt.Errorf("git release result is missing")
	}
	if ctx.SourceFormat != releaseconfig.SourceFormatV2 {
		return nil, fmt.Errorf("dispatch requests support V2 repositories only")
	}
	if ctx.Delivery != string(releaseconfig.DeliveryGitHubActions) {
		return nil, fmt.Errorf("dispatch request requires github-actions delivery, got %q", ctx.Delivery)
	}
	if strings.TrimSpace(ctx.Workflow) == "" {
		return nil, fmt.Errorf("dispatch request requires a validated github-actions workflow")
	}
	if !fullGitSHARegexp.MatchString(result.CommitSHA) {
		return nil, fmt.Errorf("dispatch request requires full release commit SHA, got %q", result.CommitSHA)
	}
	if !result.CommitCreated || !result.TagCreated {
		return nil, fmt.Errorf("dispatch request requires created release commit and unit tag")
	}
	if result.Unit != ctx.Unit.ID || result.Version != ctx.NextVersion || result.Tag != ctx.Tag {
		return nil, fmt.Errorf("git release result does not match release execution context")
	}
	if parsedVersion, ok := ctx.TagSpec.Parse(ctx.Tag); !ok || parsedVersion != ctx.NextVersion {
		return nil, fmt.Errorf("dispatch tag %q does not match unit tag spec or next version %q", ctx.Tag, ctx.NextVersion)
	}
	coordinator := NewGitReleaseCoordinator()
	tagCommit, err := coordinator.tagCommit(ctx.RepositoryRoot, ctx.Tag)
	if err != nil {
		return nil, err
	}
	if tagCommit != result.CommitSHA {
		return nil, fmt.Errorf("unit tag %q points to %s, expected release commit %s", ctx.Tag, tagCommit, result.CommitSHA)
	}
	committedVersion, err := committedUnitVersion(ctx.RepositoryRoot, result.CommitSHA, ctx.Unit.ID)
	if err != nil {
		return nil, err
	}
	if committedVersion != ctx.NextVersion {
		return nil, fmt.Errorf("committed V2 state unit %q version = %q, expected %q", ctx.Unit.ID, committedVersion, ctx.NextVersion)
	}
	remoteName := strings.TrimSpace(result.RepositoryRemoteName)
	remote := strings.TrimSpace(result.RepositoryRemote)
	if remoteName == "" || remote == "" {
		return nil, fmt.Errorf("dispatch request requires the exact V2 release remote selected by git coordination")
	}
	identity, err := newReleaseDispatchIdentity(remoteName, remote, ctx.Unit.ID, ctx.NextVersion, ctx.Tag, result.CommitSHA, ctx.Workflow, ctx.Executor, ctx.Delivery)
	if err != nil {
		return nil, err
	}
	inputs := map[string]string{
		"unit":        ctx.Unit.ID,
		"version":     ctx.NextVersion,
		"tag":         ctx.Tag,
		"release_sha": result.CommitSHA,
	}
	return &ReleaseDispatchRequest{
		RepositoryRemoteName: remoteName,
		UnitID:               ctx.Unit.ID,
		Version:              ctx.NextVersion,
		Tag:                  ctx.Tag,
		ReleaseCommitSHA:     result.CommitSHA,
		WorkflowPath:         ctx.Workflow,
		WorkflowFileName:     filepath.Base(ctx.Workflow),
		Delivery:             ctx.Delivery,
		Executor:             ctx.Executor,
		Inputs:               inputs,
		Identity:             identity,
	}, nil
}

type ReleaseDispatchDryRunSummary struct {
	Ref             string
	Inputs          map[string]string
	JournalIdentity string
	JournalLocation string
	Status          string
}

func BuildReleaseDispatchDryRunSummary(ctx *ReleaseExecutionContext) (*ReleaseDispatchDryRunSummary, error) {
	if ctx == nil {
		return nil, fmt.Errorf("release execution context is missing")
	}
	if ctx.SourceFormat != releaseconfig.SourceFormatV2 || ctx.Delivery != string(releaseconfig.DeliveryGitHubActions) {
		return nil, nil
	}
	if strings.TrimSpace(ctx.Workflow) == "" {
		return nil, fmt.Errorf("github-actions workflow is missing")
	}
	inputs := map[string]string{
		"unit":        ctx.Unit.ID,
		"version":     ctx.NextVersion,
		"tag":         ctx.Tag,
		"release_sha": "pending release commit",
	}
	return &ReleaseDispatchDryRunSummary{
		Ref:             ctx.Tag,
		Inputs:          inputs,
		JournalIdentity: "pending release commit",
		JournalLocation: "pending release commit",
		Status:          "planned after release commit and tag push",
	}, nil
}

func committedUnitVersion(repositoryRoot, commitSHA, unitID string) (string, error) {
	stateContent, err := NewGitReleaseCoordinator().gitOutput(repositoryRoot, "show", commitSHA+":"+releaseconfig.V2Directory+"/"+releaseconfig.V2StateFileName)
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

func sortedDispatchInputKeys(inputs map[string]string) []string {
	keys := make([]string, 0, len(inputs))
	for key := range inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
