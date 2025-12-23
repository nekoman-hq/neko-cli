package release

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

import (
	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/git"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

func Preflight() {
	log.V(log.Preflight, "Running pre-flight checks")
	if err := git.IsClean(); err != nil {
		errors.Error(
			"Uncommitted Changes",
			err.Error(),
			errors.ErrDirtyWorkingTree,
		)
	}

	if err := git.EnsureNotDetached(); err != nil {
		errors.Error(
			"Detached HEAD",
			err.Error(),
			errors.ErrDetachedHead,
		)
	}

	if err := git.OnMainBranch(); err != nil {
		errors.Error(
			"Incorrect Branch",
			err.Error(),
			errors.ErrWrongBranch,
		)
	}

	if err := git.HasUpstream(); err != nil {
		errors.Error(
			"No Upstream Branch",
			err.Error(),
			errors.ErrNoUpstream,
		)
	}

	if err := git.IsUpToDate(); err != nil {
		errors.Error(
			"Branch Out of Date",
			err.Error(),
			errors.ErrBranchBehind,
		)
	}

	log.V(log.Preflight, "\uF00C Preflight checks succeeded!")
}
