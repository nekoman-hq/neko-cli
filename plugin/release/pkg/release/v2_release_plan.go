package release

import "fmt"

// ReleasePlan contains the canonical active V2 release facts consumed by
// planning, execution-journal preparation, and command outcomes.
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

// BuildReleasePlan derives the active V2 release facts from the immutable
// execution context without performing I/O or mutation.
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

func ownershipSummary(capabilities ExecutorCapabilities) string {
	return fmt.Sprintf("versionFiles=%s commit=%s tag=%s push=%s githubRelease=%s",
		capabilities.VersionFilesOwner,
		capabilities.CommitOwner,
		capabilities.TagOwner,
		capabilities.PushOwner,
		capabilities.GitHubReleaseOwner,
	)
}
