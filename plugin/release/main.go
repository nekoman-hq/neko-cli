// plugin/release/main.go
package main

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"

	pluginerrors "github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/contributors"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/history"
	initcmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/init"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/migrate"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/pluginindex"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/validate"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"

	// Register all release tools
	_ "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool"
)

func main() {
	// Set plugin info for error responses
	pluginerrors.PluginName = metadata.PluginName
	pluginerrors.PluginVersion = metadata.Version

	// Read request from stdin
	var req plugin.Request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		pluginerrors.WriteError("PARSE_ERROR", fmt.Sprintf("failed to parse request: %v", err))
	}

	// Set verbose mode from request context
	log.Verbose = req.Context.Verbose

	if err := workspace.ChangeToProjectRoot(req.Context.WorkingDir); err != nil {
		pluginerrors.WriteError("WORKSPACE_ERROR", err.Error())
	}

	var resp *plugin.Response
	var err error

	switch req.Command {
	case "init":
		resp, err = initcmd.HandleInit(req)
	case "unit-add":
		resp, err = initcmd.HandleUnitAdd(req)
	case "init-options":
		resp, err = initcmd.GetAvailableOptions()
	case "migrate":
		resp, err = migrate.HandleMigrate(req)
	case "patch":
		resp, err = release.HandleRelease(req, release.Patch)
	case "minor":
		resp, err = release.HandleRelease(req, release.Minor)
	case "major":
		resp, err = release.HandleRelease(req, release.Major)
	case "resume":
		resp, err = release.HandleResume(req)
	case "history":
		resp, err = history.HandleHistory(req)
	case "contributors":
		resp, err = contributors.HandleContributors(req)
	case "validate":
		resp, err = validate.HandleValidate(req)
	case "plugin-index":
		resp, err = pluginindex.HandlePluginIndex(req)
	default:
		resp, err = nil, fmt.Errorf("unknown command: %s", req.Command)
	}

	if err != nil {
		var fatal *release.FatalCommandError
		if stderrors.As(err, &fatal) {
			pluginerrors.WriteError(fatal.Code(), fatal.Error())
		}
		pluginerrors.WriteError("EXECUTION_ERROR", err.Error())
	}

	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		pluginerrors.WriteError("RESPONSE_ERROR", fmt.Sprintf("failed to encode response: %v", err))
	}
}
