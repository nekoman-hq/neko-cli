// Package git includes operations using git or git-cli
package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/errors"
	"github.com/nekoman-hq/neko-cli/pkg/log"
	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

func LatestTag() string {
	return LatestTagAt("")
}

// LatestTagAt returns the closest tag reachable from HEAD at an explicit
// repository root.
func LatestTagAt(repositoryRoot string) string {
	log.PluginV(log.Exec, fmt.Sprintf("%s (Extract last tag)", log.ColorText(log.ColorGreen, "git describe --tags --abbrev=0")))
	cmd := gitCommandAt(repositoryRoot, "describe", "--tags", "--abbrev=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errors.WriteWarning(
			"Failed to get latest tag",
			"No tags found or could not execute git describe.\nUsing default version 0.1.0.",
		)
		return "0.1.0"
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		errors.WriteWarning(
			"No tags found",
			"No tags exist in this repository.\nUsing default version 0.1.0.",
		)
		return "0.1.0"
	}

	log.PluginV(log.Guard, fmt.Sprintf("Latest tag: %s", outputStr))
	return outputStr
}

// GetTags returns a list of all git tags
func GetTags() []string {
	return GetTagsAt("")
}

// GetTagsAt returns all git tags from an explicit repository root.
func GetTagsAt(repositoryRoot string) []string {
	return getTagsAt(repositoryRoot, true)
}

// GetTagsAtForInspection returns the same legacy tag inventory without
// lifecycle/query progress logs.
func GetTagsAtForInspection(repositoryRoot string) []string {
	return getTagsAt(repositoryRoot, false)
}

func getTagsAt(repositoryRoot string, reportProgress bool) []string {
	if reportProgress {
		log.PluginV(log.Exec, "Fetching git tags: "+
			log.ColorText(log.ColorGreen, "git tag"))
	}

	cmd := gitCommandAt(repositoryRoot, "tag")
	tagsOut, err := cmd.Output()
	if err != nil {
		errors.WriteWarning(
			"Failed to fetch tags",
			fmt.Sprintf("Command failed: %s", err.Error()),
		)
		return []string{}
	}

	tagList := strings.Split(strings.TrimSpace(string(tagsOut)), "\n")
	if len(tagList) == 1 && tagList[0] == "" {
		return []string{}
	}

	return tagList
}

// CountCommitsBetween counts commits between two references
func CountCommitsBetween(from, to string) int {
	return CountCommitsBetweenAt("", from, to)
}

// CountCommitsBetweenAt counts commits between two references from an explicit
// repository root.
func CountCommitsBetweenAt(repositoryRoot, from, to string) int {
	return countCommitsBetweenAt(repositoryRoot, from, to, true)
}

// CountCommitsBetweenAtForInspection returns the same legacy commit count
// without lifecycle/query progress logs.
func CountCommitsBetweenAtForInspection(repositoryRoot, from, to string) int {
	return countCommitsBetweenAt(repositoryRoot, from, to, false)
}

func countCommitsBetweenAt(repositoryRoot, from, to string, reportProgress bool) int {
	var cmd *exec.Cmd

	if from == "" {
		if reportProgress {
			log.PluginV(log.Exec, fmt.Sprintf("Counting commits up to %s: %s",
				to, log.ColorText(log.ColorGreen, fmt.Sprintf("git rev-list --count %s", to))))
		}
		cmd = gitCommandAt(repositoryRoot, "rev-list", "--count", to)
	} else {
		if reportProgress {
			log.PluginV(log.Exec, fmt.Sprintf("Counting commits between %s and %s: %s",
				from, to, log.ColorText(log.ColorGreen, fmt.Sprintf("git rev-list --count %s..%s", from, to))))
		}
		cmd = gitCommandAt(repositoryRoot, "rev-list", "--count", fmt.Sprintf("%s..%s", from, to))
	}

	out, err := cmd.Output()
	if err != nil {
		errors.WriteWarning(
			"Failed to count commits",
			fmt.Sprintf("Command failed for range %s..%s: %s", from, to, err.Error()),
		)
		return 0
	}

	countStr := strings.TrimSpace(string(out))
	count, err := strconv.Atoi(countStr)
	if err != nil {
		errors.WriteWarning(
			"Failed to parse commit count",
			fmt.Sprintf("Invalid count value: %s", countStr),
		)
		return 0
	}

	return count
}

// UnitTag is a tag that exactly matches a release unit TagSpec.
type UnitTag struct {
	Tag     string
	Version string
}

// LatestUnitTag returns the closest matching tag reachable from HEAD.
func LatestUnitTag(spec releaseconfig.TagSpec) (*UnitTag, error) {
	tags, err := UnitTagsInHistory(spec)
	if err != nil {
		return nil, err
	}
	if len(tags) == 0 {
		return nil, nil
	}
	return &tags[len(tags)-1], nil
}

// UnitTagsInHistory returns exact TagSpec matches in HEAD history order.
func UnitTagsInHistory(spec releaseconfig.TagSpec) ([]UnitTag, error) {
	return UnitTagsInHistoryAt("", spec)
}

// UnitTagsInHistoryAt returns exact TagSpec matches in HEAD history order from
// an explicit repository root.
func UnitTagsInHistoryAt(repositoryRoot string, spec releaseconfig.TagSpec) ([]UnitTag, error) {
	commits, err := gitOutputAt(repositoryRoot, "log", "--reverse", "--format=%H", "HEAD")
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{})
	var result []UnitTag
	for _, commit := range nonEmptyLines(commits) {
		tagsAtCommit, err := gitOutputAt(repositoryRoot, "tag", "--points-at", commit, "--list", spec.Pattern())
		if err != nil {
			return nil, err
		}
		for _, tag := range nonEmptyLines(tagsAtCommit) {
			version, ok := spec.Parse(tag)
			if !ok {
				continue
			}
			if _, exists := seen[tag]; exists {
				continue
			}
			seen[tag] = struct{}{}
			result = append(result, UnitTag{Tag: tag, Version: version})
		}
	}
	return result, nil
}

// CountCommitsBetweenPaths counts commits in a range constrained to pathspecs.
func CountCommitsBetweenPaths(from, to string, paths []string) (int, error) {
	return CountCommitsBetweenPathsAt("", from, to, paths)
}

// CountCommitsBetweenPathsAt counts commits in a range constrained to
// pathspecs from an explicit repository root.
func CountCommitsBetweenPathsAt(repositoryRoot, from, to string, paths []string) (int, error) {
	var rev string
	if from == "" {
		rev = to
	} else {
		rev = fmt.Sprintf("%s..%s", from, to)
	}
	args := []string{"rev-list", "--count", rev}
	args = append(args, gitPathspecArgs(paths)...)

	out, err := gitOutputAt(repositoryRoot, args...)
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count %q: %w", strings.TrimSpace(out), err)
	}
	return count, nil
}

// ContributorsForPaths returns contributors constrained to pathspecs.
func ContributorsForPaths(paths []string) ([]Contributor, error) {
	return ContributorsForPathsAt("", paths)
}

// ContributorsForPathsAt returns contributors constrained to pathspecs from an
// explicit repository root.
func ContributorsForPathsAt(repositoryRoot string, paths []string) ([]Contributor, error) {
	return contributorsForPathsAt(repositoryRoot, paths, true)
}

// ContributorsForPathsAtForInspection returns the same path-constrained
// contributors without lifecycle/query progress logs.
func ContributorsForPathsAtForInspection(repositoryRoot string, paths []string) ([]Contributor, error) {
	return contributorsForPathsAt(repositoryRoot, paths, false)
}

func contributorsForPathsAt(repositoryRoot string, paths []string, reportProgress bool) ([]Contributor, error) {
	args := []string{"shortlog", "-sne", "HEAD"}
	args = append(args, gitPathspecArgs(paths)...)

	out, err := gitOutputAt(repositoryRoot, args...)
	if err != nil {
		return nil, err
	}
	return parseContributorsWithProgress(out, reportProgress), nil
}

func gitPathspecArgs(paths []string) []string {
	if len(paths) == 0 || (len(paths) == 1 && paths[0] == "**") {
		return nil
	}
	args := []string{"--"}
	for _, path := range paths {
		if path == "**" {
			continue
		}
		args = append(args, ":(glob)"+path)
	}
	if len(args) == 1 {
		return nil
	}
	return args
}

func gitOutputAt(repositoryRoot string, args ...string) (string, error) {
	cmd := gitCommandAt(repositoryRoot, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func gitCommandAt(repositoryRoot string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	if strings.TrimSpace(repositoryRoot) != "" {
		cmd.Dir = repositoryRoot
	}
	return cmd
}

func nonEmptyLines(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}
