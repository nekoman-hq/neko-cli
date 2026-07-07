package release

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// MaterializationPlan describes release-version file changes before anything is
// committed, tagged, pushed, or published.
//
//nolint:govet // Logical materialization-domain order is clearer than fieldalignment ordering here.
type MaterializationPlan struct {
	Unit           releaseconfig.ReleaseUnit
	RepositoryRoot string
	UnitRoot       string
	CurrentVersion string
	NextVersion    string
	Tag            string
	Executor       string
	Changes        []MaterializedFileChange
	BlockedReason  string
}

// MaterializedFileChange is a structured, auditable file mutation.
//
//nolint:govet // Logical file-change order is clearer than fieldalignment ordering here.
type MaterializedFileChange struct {
	AbsolutePath             string
	RepositoryRelativePath   string
	BeforeContent            []byte
	BeforeHash               string
	AfterContent             []byte
	Reason                   string
	FileMode                 os.FileMode
	RequiredForReleaseCommit bool
	Existed                  bool
	RepositoryWide           bool
}

func newMaterializationPlan(ctx *ReleaseExecutionContext) MaterializationPlan {
	return MaterializationPlan{
		Unit:           ctx.Unit,
		RepositoryRoot: ctx.RepositoryRoot,
		UnitRoot:       ctx.UnitRoot,
		CurrentVersion: ctx.CurrentVersion,
		NextVersion:    ctx.NextVersion,
		Tag:            ctx.Tag,
		Executor:       ctx.Executor,
	}
}

func newMaterializedFileChange(ctx *ReleaseExecutionContext, absolutePath string, beforeContent, afterContent []byte, mode os.FileMode, existed bool, reason string, required bool) (MaterializedFileChange, error) {
	absolutePath, err := filepath.Abs(absolutePath)
	if err != nil {
		return MaterializedFileChange{}, fmt.Errorf("materialized path %q cannot be resolved: %w", absolutePath, err)
	}
	repositoryRelativePath, err := filepath.Rel(ctx.RepositoryRoot, absolutePath)
	if err != nil {
		return MaterializedFileChange{}, fmt.Errorf("materialized path %q cannot be related to repository root: %w", absolutePath, err)
	}
	hash := sha256.Sum256(beforeContent)
	return MaterializedFileChange{
		AbsolutePath:             absolutePath,
		RepositoryRelativePath:   repositoryRelativePath,
		BeforeContent:            append([]byte(nil), beforeContent...),
		BeforeHash:               fmt.Sprintf("%x", hash[:]),
		AfterContent:             append([]byte(nil), afterContent...),
		Reason:                   reason,
		FileMode:                 mode,
		RequiredForReleaseCommit: required,
		Existed:                  existed,
	}, nil
}
