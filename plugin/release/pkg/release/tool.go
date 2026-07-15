//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package release

//lint:file-ignore SA1019 V1 compatibility release paths intentionally use deprecated V1 APIs during migration

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
	config2 "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      18.12.2025
*/

type Tool interface {
	Name() string
	Init(cfg *config2.V1ReleaseConfig) error
	Execute(ctx *ReleaseExecutionContext) error
	ValidateRequirements(ctx *ReleaseExecutionContext) error
	ResolveFiles(ctx *ReleaseExecutionContext) ([]string, error)
	Release(v *semver.Version) error
	RevertRelease() error
}

type ToolBase struct{}

func (tb *ToolBase) ValidateRequirements(ctx *ReleaseExecutionContext) error {
	return ValidateRequirementsForContext(ctx)
}

func (tb *ToolBase) ResolveFiles(ctx *ReleaseExecutionContext) ([]string, error) {
	if ctx == nil {
		return nil, fmt.Errorf("release execution context is missing")
	}
	return requiredReleaseSystemFiles(ctx.Executor)
}

func (tb *ToolBase) InUnitRoot(ctx *ReleaseExecutionContext, fn func() error) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to resolve current working directory: %w", err)
	}
	if err := os.Chdir(ctx.UnitRoot); err != nil {
		return fmt.Errorf("failed to enter unit root %s: %w", ctx.UnitRoot, err)
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()
	return fn()
}

func (tb *ToolBase) RequireBinary(name string) error {
	return NewSystemV1BinaryLocator().Require(name)
}

type GitReleaseState struct {
	PreHead              string
	ReleaseHead          string
	TagName              string
	GitHubReleaseTag     string // usually same as TagName
	PreviousVersion      string
	PushedCommit         bool
	PushedTag            bool
	CreatedGitHubRelease bool
	UpdatedConfig        bool
}

func (st GitReleaseState) hasMutatingStep() bool {
	return st.ReleaseHead != "" ||
		st.TagName != "" ||
		st.PushedCommit ||
		st.PushedTag ||
		st.CreatedGitHubRelease ||
		st.UpdatedConfig
}

func (tb *ToolBase) RevertGitRelease(st GitReleaseState) error {
	return NewSystemV1ReleaseRollback().Rollback("", st)
}

func (tb *ToolBase) DeleteGitHubRelease(tag string) error {
	return newSystemV1GitHubReleaseRemover().Delete("", tag)
}

// CreateReleaseCommit creates the chore commit for the release
func (tb *ToolBase) CreateReleaseCommit(v *semver.Version) error {
	return NewSystemV1GitWriter().CreateReleaseCommit("", v)
}

// CreateGitTag creates a git tag for the version
func (tb *ToolBase) CreateGitTag(v *semver.Version) error {
	return NewSystemV1GitWriter().CreateGitTag("", v)
}

// PushCommits pushes the release commit to remote
func (tb *ToolBase) PushCommits() error {
	return NewSystemV1GitWriter().PushCommits("")
}

// PushGitTag pushes the git tag to remote
func (tb *ToolBase) PushGitTag(v *semver.Version) error {
	return NewSystemV1GitWriter().PushGitTag("", v)
}
