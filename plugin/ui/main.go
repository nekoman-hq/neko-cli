package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	addpkg "github.com/nekoman-hq/neko-cli/plugin/ui/pkg/add"
	initpkg "github.com/nekoman-hq/neko-cli/plugin/ui/pkg/init"
	listpkg "github.com/nekoman-hq/neko-cli/plugin/ui/pkg/list"
	"github.com/nekoman-hq/neko-cli/plugin/ui/pkg/metadata"
	removepkg "github.com/nekoman-hq/neko-cli/plugin/ui/pkg/remove"
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

	var resp *plugin.Response
	var err error

	// Route to command handlers
	switch req.Command {
	case "init":
		resp, err = initpkg.Handle(req)
	case "list":
		resp, err = listpkg.Handle(req)
	case "add":
		resp, err = addpkg.Handle(req)
	case "remove":
		resp, err = removepkg.Handle(req)
	default:
		resp, err = nil, fmt.Errorf("unknown command: %s", req.Command)
	}

	if err != nil {
		errors.WriteError("EXECUTION_ERROR", err.Error())
	}

	// Write response to stdout
	if err := json.NewEncoder(os.Stdout).Encode(resp); err != nil {
		errors.WriteError("RESPONSE_ERROR", fmt.Sprintf("failed to encode response: %v", err))
	}
}
