package release

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"gopkg.in/yaml.v3"
)

type VersionMaterializer interface {
	Plan(ctx *ReleaseExecutionContext) (*MaterializationPlan, error)
	Validate(plan *MaterializationPlan) error
}

func ResolveVersionMaterializer(executor string) (VersionMaterializer, error) {
	switch releaseconfig.ExecutorType(executor) {
	case releaseconfig.ExecutorGoReleaser:
		return GoReleaserMaterializer{}, nil
	case releaseconfig.ExecutorJReleaser:
		return JReleaserMaterializer{}, nil
	case releaseconfig.ExecutorReleaseIt:
		return ReleaseItMaterializer{}, nil
	default:
		return nil, fmt.Errorf("unknown executor: %s", executor)
	}
}

type GoReleaserMaterializer struct{}

func (GoReleaserMaterializer) Plan(ctx *ReleaseExecutionContext) (*MaterializationPlan, error) {
	plan := newMaterializationPlan(ctx)
	if err := appendPluginReleaseMaterialization(ctx, &plan); err != nil {
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
	after, err := materializeJReleaserVersion(before, ctx.NextVersion)
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
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0644, false, nil
		}
		return nil, 0, false, fmt.Errorf("inspect materialized file %s: %w", path, err)
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

func materializeJReleaserVersion(content []byte, nextVersion string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", jReleaserConfigFile, err)
	}
	versionNode, err := findYAMLPath(&doc, "project", "version")
	if err != nil {
		return nil, fmt.Errorf("locate project.version in %s: %w", jReleaserConfigFile, err)
	}
	lines := strings.SplitAfter(string(content), "\n")
	if versionNode.Line < 1 || versionNode.Line > len(lines) {
		return nil, fmt.Errorf("project.version line %d is outside %s", versionNode.Line, jReleaserConfigFile)
	}
	line := lines[versionNode.Line-1]
	prefixIndex := strings.Index(line, "version:")
	if prefixIndex < 0 {
		return nil, fmt.Errorf("project.version line does not contain version key")
	}
	lineEnding := ""
	if strings.HasSuffix(line, "\n") {
		lineEnding = "\n"
		line = strings.TrimSuffix(line, "\n")
	}
	if strings.HasSuffix(line, "\r") {
		lineEnding = "\r" + lineEnding
		line = strings.TrimSuffix(line, "\r")
	}
	prefix := line[:prefixIndex+len("version:")]
	lines[versionNode.Line-1] = prefix + " " + nextVersion + lineEnding
	return []byte(strings.Join(lines, "")), nil
}

func findYAMLPath(doc *yaml.Node, path ...string) (*yaml.Node, error) {
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("document is empty")
	}
	node := doc.Content[0]
	for _, part := range path {
		if node.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s is not a mapping", part)
		}
		var next *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == part {
				next = node.Content[i+1]
				break
			}
		}
		if next == nil {
			return nil, fmt.Errorf("%s not found", part)
		}
		node = next
	}
	return node, nil
}
