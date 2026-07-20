package release

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
	jreleaserconfig "github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool/jreleaser"
)

type VersionMaterializer interface {
	Plan(ctx *ReleaseExecutionContext) (*MaterializationPlan, error)
	Validate(plan *MaterializationPlan) error
}

func ResolveVersionMaterializer(executor string) (VersionMaterializer, error) {
	identity, err := releasetool.ParseIdentity(executor)
	if err != nil {
		return nil, fmt.Errorf("unknown executor: %s", executor)
	}
	switch identity {
	case releasetool.GoReleaser:
		return GoReleaserMaterializer{}, nil
	case releasetool.JReleaser:
		return JReleaserMaterializer{}, nil
	case releasetool.ReleaseIt:
		return ReleaseItMaterializer{}, nil
	default:
		return nil, fmt.Errorf("unknown executor: %s", executor)
	}
}

type GoReleaserMaterializer struct{}

func (GoReleaserMaterializer) Plan(ctx *ReleaseExecutionContext) (*MaterializationPlan, error) {
	plan := newMaterializationPlan(ctx)
	if err := appendPluginManifestMaterialization(ctx, &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

func (GoReleaserMaterializer) Validate(plan *MaterializationPlan) error {
	return ValidateMaterializationPlan(plan)
}

type ReleaseItMaterializer struct{}

func (ReleaseItMaterializer) Plan(ctx *ReleaseExecutionContext) (*MaterializationPlan, error) {
	plan := newMaterializationPlan(ctx)
	plan.BlockedReason = ctx.Capabilities.V2LocalExecutionBlockedReason
	return &plan, nil
}

func (ReleaseItMaterializer) Validate(plan *MaterializationPlan) error {
	return ValidateMaterializationPlan(plan)
}

type JReleaserMaterializer struct{}

func (JReleaserMaterializer) Plan(ctx *ReleaseExecutionContext) (*MaterializationPlan, error) {
	plan := newMaterializationPlan(ctx)
	path := filepath.Join(ctx.UnitRoot, jReleaserConfigFile)
	before, mode, existed, err := readMaterializedFile(path)
	if err != nil {
		return nil, err
	}
	if !existed {
		return nil, fmt.Errorf("%s not found", jReleaserConfigFile)
	}
	after, err := jreleaserconfig.RewriteProjectVersion(before, ctx.NextVersion)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(before, after) {
		return &plan, nil
	}
	change, err := newMaterializedFileChange(
		ctx,
		path,
		before,
		after,
		mode,
		existed,
		"sync JReleaser project.version with release plan",
		true,
	)
	if err != nil {
		return nil, err
	}
	plan.Changes = append(plan.Changes, change)
	return &plan, nil
}

func (JReleaserMaterializer) Validate(plan *MaterializationPlan) error {
	return ValidateMaterializationPlan(plan)
}

func ValidateMaterializationPlan(plan *MaterializationPlan) error {
	if plan == nil {
		return fmt.Errorf("materialization plan is missing")
	}
	seen := make(map[string]struct{}, len(plan.Changes))
	for _, change := range plan.Changes {
		if strings.TrimSpace(change.Reason) == "" {
			return fmt.Errorf("materialization change %s has no reason", change.AbsolutePath)
		}
		if !filepath.IsAbs(change.AbsolutePath) {
			return fmt.Errorf("materialization change %s must be absolute", change.AbsolutePath)
		}
		if change.RepositoryRelativePath == "" || filepath.IsAbs(change.RepositoryRelativePath) {
			return fmt.Errorf("materialization change %s has invalid repository-relative path %q", change.AbsolutePath, change.RepositoryRelativePath)
		}
		if err := ensureInsideRepository(plan.RepositoryRoot, change.AbsolutePath, plan.Unit.ID); err != nil {
			return err
		}
		if !change.RepositoryWide {
			relToUnit, err := filepath.Rel(plan.UnitRoot, change.AbsolutePath)
			if err != nil {
				return fmt.Errorf("materialization change %s cannot be related to unit root: %w", change.AbsolutePath, err)
			}
			if relToUnit == ".." || strings.HasPrefix(relToUnit, ".."+string(filepath.Separator)) || filepath.IsAbs(relToUnit) {
				return fmt.Errorf("materialization change %s is outside unit root %s", change.AbsolutePath, plan.UnitRoot)
			}
		}
		if _, ok := seen[change.AbsolutePath]; ok {
			return fmt.Errorf("materialization plan contains duplicate target %s", change.AbsolutePath)
		}
		seen[change.AbsolutePath] = struct{}{}
	}
	return nil
}

func readMaterializedFile(path string) ([]byte, os.FileMode, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0644, false, nil
		}
		return nil, 0, false, fmt.Errorf("inspect materialized file %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, false, fmt.Errorf("materialized file %s is a symlink", path)
	}
	if info.IsDir() {
		return nil, 0, false, fmt.Errorf("materialized file %s is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, fmt.Errorf("read materialized file %s: %w", path, err)
	}
	return data, info.Mode().Perm(), true, nil
}
