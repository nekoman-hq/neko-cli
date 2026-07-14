package release

import "github.com/nekoman-hq/neko-cli/pkg/plugin"

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

func commandFlagBool(flags map[string]any, name string) bool {
	value, ok := flags[name].(bool)
	return ok && value
}

func commandFlagString(flags map[string]any, name string) string {
	value, _ := flags[name].(string)
	return value
}
