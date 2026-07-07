package release

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const v2GitCoordinationUnavailableMessage = "V2 Git release coordination is prepared, but V2 publication adapters are not available yet. No release state, commit, tag, push, or publish operation was performed."

type V2GitOwnership struct {
	VersionAuthority       string
	VersionMaterialization string
	State                  string
	Commit                 string
	Tag                    string
	Push                   string
	Publish                string
}

func NewV2GitOwnership() V2GitOwnership {
	return V2GitOwnership{
		VersionAuthority:       "neko-cli",
		VersionMaterialization: "neko-cli",
		State:                  "neko-cli",
		Commit:                 "neko-cli",
		Tag:                    "neko-cli",
		Push:                   "neko-cli",
		Publish:                "future publish-only adapter",
	}
}

func (ownership V2GitOwnership) Summary() string {
	return fmt.Sprintf("versionAuthority=%s materialization=%s state=%s commit=%s tag=%s push=%s publish=%s",
		ownership.VersionAuthority,
		ownership.VersionMaterialization,
		ownership.State,
		ownership.Commit,
		ownership.Tag,
		ownership.Push,
		ownership.Publish,
	)
}

type KnownReleaseFile struct {
	AbsolutePath           string
	RepositoryRelativePath string
}

type KnownReleaseFiles struct {
	RepositoryRoot string
	Files          []KnownReleaseFile
}

func NewKnownReleaseFiles(ctx *ReleaseExecutionContext, materializationPlan *MaterializationPlan) (KnownReleaseFiles, error) {
	if ctx == nil {
		return KnownReleaseFiles{}, fmt.Errorf("release execution context is missing")
	}
	files := KnownReleaseFiles{RepositoryRoot: ctx.RepositoryRoot}
	if err := files.AddAbsolute(releaseconfig.V2StatePath(ctx.RepositoryRoot)); err != nil {
		return KnownReleaseFiles{}, err
	}
	if materializationPlan != nil {
		for _, change := range materializationPlan.Changes {
			if change.RequiredForReleaseCommit {
				if err := files.AddAbsolute(change.AbsolutePath); err != nil {
					return KnownReleaseFiles{}, err
				}
			}
		}
	}
	if err := files.Validate(); err != nil {
		return KnownReleaseFiles{}, err
	}
	return files, nil
}

func (files *KnownReleaseFiles) AddAbsolute(absolutePath string) error {
	file, err := newKnownReleaseFile(files.RepositoryRoot, absolutePath)
	if err != nil {
		return err
	}
	files.Files = append(files.Files, file)
	return nil
}

func (files KnownReleaseFiles) Validate() error {
	if strings.TrimSpace(files.RepositoryRoot) == "" {
		return fmt.Errorf("repository root is missing")
	}
	seen := make(map[string]struct{}, len(files.Files))
	for _, file := range files.Files {
		normalized, err := newKnownReleaseFile(files.RepositoryRoot, file.AbsolutePath)
		if err != nil {
			return err
		}
		if file.RepositoryRelativePath != "" && file.RepositoryRelativePath != normalized.RepositoryRelativePath {
			return fmt.Errorf("known release file %s has inconsistent repository-relative path %q", file.AbsolutePath, file.RepositoryRelativePath)
		}
		if _, ok := seen[normalized.RepositoryRelativePath]; ok {
			return fmt.Errorf("known release file %s is declared more than once", normalized.RepositoryRelativePath)
		}
		seen[normalized.RepositoryRelativePath] = struct{}{}
	}
	return nil
}

func (files KnownReleaseFiles) RelativePaths() []string {
	paths := make([]string, 0, len(files.Files))
	for _, file := range files.Files {
		paths = append(paths, filepath.ToSlash(file.RepositoryRelativePath))
	}
	sort.Strings(paths)
	return paths
}

func (files KnownReleaseFiles) RelativePathSet() map[string]struct{} {
	paths := files.RelativePaths()
	set := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		set[path] = struct{}{}
	}
	return set
}

func newKnownReleaseFile(repositoryRoot, absolutePath string) (KnownReleaseFile, error) {
	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return KnownReleaseFile{}, fmt.Errorf("repository root %q cannot be resolved: %w", repositoryRoot, err)
	}
	absoluteFile, err := filepath.Abs(absolutePath)
	if err != nil {
		return KnownReleaseFile{}, fmt.Errorf("known release file %q cannot be resolved: %w", absolutePath, err)
	}
	relativePath, err := filepath.Rel(absoluteRoot, absoluteFile)
	if err != nil {
		return KnownReleaseFile{}, fmt.Errorf("known release file %s cannot be related to repository root: %w", absoluteFile, err)
	}
	if relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
		return KnownReleaseFile{}, fmt.Errorf("known release file %s is outside repository root %s", absoluteFile, absoluteRoot)
	}
	return KnownReleaseFile{
		AbsolutePath:           absoluteFile,
		RepositoryRelativePath: filepath.ToSlash(relativePath),
	}, nil
}

// GitReleaseResult records how far Neko-owned V2 Git coordination progressed.
//
//nolint:govet // Release result fields follow the release lifecycle order.
type GitReleaseResult struct {
	Unit              string
	Version           string
	Tag               string
	CommitSHA         string
	CommitCreated     bool
	TagCreated        bool
	CommitPushed      bool
	TagPushed         bool
	ReachedPhase      string
	KnownReleaseFiles []string
	RecoveryGuidance  string
}

func newGitReleaseResult(ctx *ReleaseExecutionContext, files KnownReleaseFiles) *GitReleaseResult {
	return &GitReleaseResult{
		Unit:              ctx.Unit.ID,
		Version:           ctx.NextVersion,
		Tag:               ctx.Tag,
		ReachedPhase:      string(ExecutionPhasePlanned),
		KnownReleaseFiles: files.RelativePaths(),
		RecoveryGuidance:  recoveryGuidanceBeforeCommit(),
	}
}

func recoveryGuidanceBeforeCommit() string {
	return "No release commit was created. The state and materialization transactions may restore only their own snapshots and unstage only their known files."
}

func recoveryGuidanceCommitCreated(commitSHA, unit, tag string) string {
	return fmt.Sprintf("Release commit %s exists locally for unit %q and tag %q. No destructive rollback was attempted; inspect the commit before retrying push or dispatch.", commitSHA, unit, tag)
}

func recoveryGuidanceCommitPushedTagMissing(commitSHA, unit, tag string) string {
	return fmt.Sprintf("Release commit %s for unit %q was pushed, but tag %q was not pushed. Do not delete remote state automatically; inspect the remote commit and push the exact tag when safe.", commitSHA, unit, tag)
}

func recoveryGuidanceComplete(commitSHA, unit, tag string) string {
	return fmt.Sprintf("Release commit %s and tag %q for unit %q were pushed. Later dispatch can target this exact tag.", commitSHA, tag, unit)
}
