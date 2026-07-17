// Package git includes operations using git or git-cli
package git

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	github "github.com/nekoman-hq/neko-cli/pkg/git"
	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type Contributor struct {
	Commits string
	Author  string
}

func Fetch() {
	log.PluginV(log.Guard, fmt.Sprintf("%s (Updating repository information)",
		log.ColorText(log.ColorGreen, "git fetch"),
	))

	_ = exec.Command("git", "fetch").Run()
}

// Current checks if a git repository exists and returns owner and repo name
func Current() (*github.RepoInfo, error) {
	log.PluginV(log.Config, fmt.Sprintf("%s (Checking Repository Origin)",
		log.ColorText(log.ColorGreen, "git remote -v"),
	))

	cmd := exec.Command("git", "remote", "-v")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"not a Git Repository: %w", err,
		)
	}

	outputStr := string(output)
	if strings.TrimSpace(outputStr) == "" {
		return nil, errors.New(
			"no Remote Found: This git repository has no remote configured.\nAdd a remote with: git remote add origin <url>",
		)
	}
	return parseRemote(outputStr)
}

// parseRemote extracts owner and repo from git remote output
func parseRemote(remoteOutput string) (*github.RepoInfo, error) {
	// Regex patterns for both SSH and HTTPS URLs
	// SSH: git@git.com:owner/repo.git
	sshPattern := regexp.MustCompile(`git@github\.com:([^/]+)/([^/\s]+?)(?:\.git)?(?:\s|$)`)
	// HTTPS: https://github.com/owner/repo.git
	httpsPattern := regexp.MustCompile(`https://github\.com/([^/]+)/([^/\s]+?)(?:\.git)?(?:\s|$)`)

	// Try SSH pattern first
	if matches := sshPattern.FindStringSubmatch(remoteOutput); len(matches) >= 3 {
		repoPath := fmt.Sprintf("%s/%s", matches[1], matches[2])
		log.PluginV(log.Config, fmt.Sprintf("Found repository: %s (SSH)",
			log.ColorText(log.ColorGreen, repoPath)))
		return &github.RepoInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	// Try HTTPS pattern
	if matches := httpsPattern.FindStringSubmatch(remoteOutput); len(matches) >= 3 {
		repoPath := fmt.Sprintf("%s/%s", matches[1], matches[2])
		log.PluginV(log.Config, fmt.Sprintf("Found repository: %s (HTTPS)",
			log.ColorText(log.ColorGreen, repoPath)))
		return &github.RepoInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	return nil, errors.New(
		"invalid Remote URL: Could not parse GitHub repository information from remote.\nOnly GitHub repositories are supported",
	)
}

func IsClean() error {
	log.PluginV(log.Preflight, fmt.Sprintf("%s (Check branch state)",
		log.ColorText(log.ColorGreen, "git status --porcelain"),
	))
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to check git status: %w", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("the working tree has uncommitted changes. Please commit or stash them")
	}

	log.PluginV(log.Preflight, "Working tree is clean")
	return nil
}

func EnsureNotDetached() error {
	log.PluginV(log.Preflight, fmt.Sprintf("%s (Ensure branch is not detached)",
		log.ColorText(log.ColorGreen, "git rev-parse --abbrev-ref HEAD"),
	))
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to determine HEAD state: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		return fmt.Errorf("detached HEAD state detected. Please checkout a branch")
	}

	log.PluginV(log.Preflight, fmt.Sprintf("HEAD attached to branch %s", log.ColorText(log.ColorGreen, branch)))
	return nil
}

func OnMainBranch() error {
	log.PluginV(log.Preflight, fmt.Sprintf("%s (Check on main branch)",
		log.ColorText(log.ColorGreen, "git rev-parse --abbrev-ref HEAD"),
	))

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to determine current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch != "main" && branch != "master" {
		return fmt.Errorf("you are on branch '%s'. Releases are only allowed from 'main' or 'master'", branch)
	}

	log.PluginV(log.Preflight, fmt.Sprintf("On %s branch", log.ColorText(log.ColorGreen, branch)))
	return nil
}

func HasUpstream() error {
	log.PluginV(log.Preflight, fmt.Sprintf("%s (Check upstream configuration)",
		log.ColorText(log.ColorGreen, "git for-each-ref"),
	))

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to determine current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))

	cmd = exec.Command(
		"git",
		"for-each-ref",
		"--format=%(upstream:short)",
		fmt.Sprintf("refs/heads/%s", branch),
	)

	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to determine upstream branch: %w", err)
	}

	upstream := strings.TrimSpace(string(output))
	if upstream == "" {
		return fmt.Errorf("branch '%s' has no upstream configured", branch)
	}

	log.PluginV(log.Preflight, fmt.Sprintf("Upstream branch: %s", log.ColorText(log.ColorGreen, upstream)))
	return nil
}

func IsUpToDate() error {
	log.PluginV(log.Preflight, fmt.Sprintf("%s (Check if branch is up to date)",
		log.ColorText(log.ColorGreen, "git status -sb"),
	))

	cmd := exec.Command("git", "status", "-sb")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to check branch status: %w", err)
	}

	status := string(output)

	if strings.Contains(status, "behind") {
		return fmt.Errorf("branch is behind its upstream. Please pull the latest changes")
	}

	log.PluginV(log.Preflight, "Branch is up to date with upstream")
	return nil
}

// CurrentBranch returns the name of the current branch
func CurrentBranch() (string, error) {
	log.PluginV(log.Exec, "Fetching current branch: "+
		log.ColorText(log.ColorGreen, "git rev-parse --abbrev-ref HEAD"))

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(
			"failed to get current branch: %w", err,
		)
	}

	branch := strings.TrimSpace(string(branchOut))
	return branch, nil
}

// Contributors returns a list of contributors with their commit counts
func Contributors() ([]Contributor, error) {
	return ContributorsAt("")
}

// ContributorsAt returns a list of contributors with their commit counts from
// an explicit repository root.
func ContributorsAt(repositoryRoot string) ([]Contributor, error) {
	log.PluginV(log.Exec, "Fetching contributors: "+
		log.ColorText(log.ColorGreen, "git shortlog -sne HEAD"))

	cmd := gitCommandAt(repositoryRoot, "shortlog", "-sne", "HEAD")
	contrib, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch contributors: %w", err,
		)
	}

	contribLines := strings.Split(strings.TrimSpace(string(contrib)), "\n")
	log.PluginV(log.Exec, fmt.Sprintf("Found %d contributors", len(contribLines)))

	return parseContributors(string(contrib)), nil
}

func parseContributors(output string) []Contributor {
	contribLines := strings.Split(strings.TrimSpace(output), "\n")
	contributors := make([]Contributor, 0, len(contribLines))
	for _, line := range contribLines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			log.PluginV(log.Exec, fmt.Sprintf("Skipping invalid contributor line: %s", line))
			continue
		}

		contributors = append(contributors, Contributor{
			Commits: parts[0],
			Author:  strings.Join(parts[1:], " "),
		})
	}

	return contributors
}
