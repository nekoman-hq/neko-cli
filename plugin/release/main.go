// plugin/release/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/contributors"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/history"
	initcmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/init"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/validate"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"

	// Register all release tools
	_ "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool"
)

func main() {
	// Set plugin info for error responses
	errors.PluginName = metadata.PluginName
	errors.PluginVersion = metadata.Version

	// Read request from stdin
	var req plugin.Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		errors.WriteError("PARSE_ERROR", fmt.Sprintf("failed to parse request: %v", err))
	}

	// Set verbose mode from request context
	log.Verbose = req.Context.Verbose

	if err := workspace.ChangeToProjectRoot(req.Context.WorkingDir); err != nil {
		errors.WriteError("WORKSPACE_ERROR", err.Error())
	}

	var resp *plugin.Response
	var err error

	switch req.Command {
	case "init":
		resp, err = initcmd.HandleInit(req)
	case "init-options":
		resp, err = initcmd.GetAvailableOptions()
	case "patch":
		resp, err = release.HandleRelease(req, release.Patch)
	case "minor":
		resp, err = release.HandleRelease(req, release.Minor)
	case "major":
		resp, err = release.HandleRelease(req, release.Major)
	case "history":
		resp, err = history.HandleHistory(req)
	case "contributors":
		resp, err = contributors.HandleContributors(req)
	case "validate":
		resp, err = validate.HandleValidate(req)
	default:
		resp, err = nil, fmt.Errorf("unknown command: %s", req.Command)
	}

	if err != nil {
		errors.WriteError("EXECUTION_ERROR", err.Error())
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		errors.WriteError("RESPONSE_ERROR", fmt.Sprintf("failed to encode response: %v", err))
	}
}
