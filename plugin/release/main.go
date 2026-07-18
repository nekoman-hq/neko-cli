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
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/evidence"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/history"
	initcmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/init"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/migrate"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/pluginindex"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/goreleaser"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/jreleaser"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/releaseit"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/validate"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
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

	root, err := resolveRequestRoot(req)
	if err != nil {
		pluginerrors.WriteError("WORKSPACE_ERROR", err.Error())
	}

	v1Executors := []release.V1Executor{
		goreleaser.NewV1Executor(),
		jreleaser.NewV1Executor(),
		releaseit.NewV1Executor(),
	}

	resp, err := handleRequestAt(root, req, v1Executors)
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

func resolveRequestRoot(req plugin.Request) (workspace.RepositoryRoot, error) {
	if req.Command == "doctor" {
		return workspace.ResolveInspectionRepositoryRoot(req.Context.WorkingDir)
	}
	return workspace.ResolveRepositoryRoot(req.Context.WorkingDir)
}

func handleRequestAt(root workspace.RepositoryRoot, req plugin.Request, v1Executors []release.V1Executor) (*plugin.Response, error) {
	switch req.Command {
	case "init":
		return initcmd.HandleInitAt(root, req)
	case "unit-add":
		return initcmd.HandleUnitAddAt(root, req)
	case "init-options":
		return initcmd.GetAvailableOptions()
	case "migrate":
		return migrate.HandleMigrate(req)
	case "patch":
		return release.HandleReleaseWithV1ExecutorsAt(root, req, release.Patch, v1Executors...)
	case "minor":
		return release.HandleReleaseWithV1ExecutorsAt(root, req, release.Minor, v1Executors...)
	case "major":
		return release.HandleReleaseWithV1ExecutorsAt(root, req, release.Major, v1Executors...)
	case "plan":
		return release.HandlePlanAt(root, req)
	case "doctor":
		return release.HandleDoctorAt(root, req)
	case "ci-validate-context":
		return release.HandleReleaseContextValidationAt(root, req)
	case "github-workflow-init":
		return release.HandleGitHubWorkflowInitAt(root, req)
	case "resume":
		return release.HandleResumeAt(root, req)
	case "evidence":
		return evidence.HandleEvidenceAt(root, req)
	case "evidence-archive":
		return evidence.HandleEvidenceArchiveAt(root, req)
	case "history":
		return history.HandleHistoryAt(root, req)
	case "contributors":
		return contributors.HandleContributorsAt(root, req)
	case "validate":
		return validate.HandleValidateAt(root, req)
	case "plugin-index":
		return pluginindex.HandlePluginIndexAt(root, req)
	default:
		return nil, fmt.Errorf("unknown command: %s", req.Command)
	}
}
