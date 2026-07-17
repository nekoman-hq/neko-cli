package release

import (
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// ReleaseCommandRequest is the typed input accepted by release application code.
type ReleaseCommandRequest struct {
	ReleaseType Type
	UnitID      string
	DryRun      bool
}

// ResumeCommandRequest is the typed input accepted by resume application code.
type ResumeCommandRequest struct {
	UnitID string
	DryRun bool
}

// PlanCommandRequest is the typed input accepted by release-plan inspection.
type PlanCommandRequest struct {
	ReleaseType Type
	UnitID      string
}

// ParseReleaseCommandRequest isolates the plugin transport's untyped flag map.
// Unknown or malformed flag values retain the command's existing zero-value
// defaults; rejecting them here would introduce a new command error contract.
func ParseReleaseCommandRequest(req plugin.Request, releaseType Type) ReleaseCommandRequest {
	return ReleaseCommandRequest{
		ReleaseType: releaseType,
		UnitID:      commandFlagString(req.Flags, "unit"),
		DryRun:      commandFlagBool(req.Flags, "dry-run"),
	}
}

// ParseResumeCommandRequest isolates the plugin transport's untyped flag map.
func ParseResumeCommandRequest(req plugin.Request) ResumeCommandRequest {
	return ResumeCommandRequest{
		UnitID: commandFlagString(req.Flags, "unit"),
		DryRun: commandFlagBool(req.Flags, "dry-run"),
	}
}

// ParsePlanCommandRequest isolates the typed release-plan inspection request.
func ParsePlanCommandRequest(req plugin.Request) (PlanCommandRequest, *CommandFailure) {
	change := strings.TrimSpace(commandFlagString(req.Flags, "change"))
	if change == "" {
		return PlanCommandRequest{}, failureFromMessage(
			"INVALID_RELEASE_CHANGE",
			"release plan inspection requires --change with one of: patch, minor, major",
		)
	}
	releaseType, err := ParseReleaseType(change)
	if err != nil {
		return PlanCommandRequest{}, failureFromError(
			"INVALID_RELEASE_CHANGE",
			fmt.Errorf("invalid release change %q: %w", change, err),
		)
	}
	return PlanCommandRequest{
		ReleaseType: releaseType,
		UnitID:      commandFlagString(req.Flags, "unit"),
	}, nil
}

func commandFlagBool(flags map[string]any, name string) bool {
	value, ok := flags[name].(bool)
	return ok && value
}

func commandFlagString(flags map[string]any, name string) string {
	value, _ := flags[name].(string)
	return value
}
