package git

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/nekoman-hq/neko-cli/internal/errors"
	"github.com/nekoman-hq/neko-cli/internal/log"
)

type RepoInfo struct {
	Owner string
	Repo  string
}

func Fetch() {
	log.V("V$ git fetch (Updating repository information)")
	exec.Command("git", "fetch")
}

// Current checks if a git repository exists and returns owner and repo name
func Current() (*RepoInfo, error) {
	command := exec.Command("git", "remote", "-v")
	output, err := command.CombinedOutput()

	if err != nil {
		errors.Fatal(
			"Not a Git Repository",
			"This directory is not a git repository.\nPlease run this command from within a git repository.",
			errors.ErrNoGitRepo,
		)
	}

	outputStr := string(output)
	if strings.TrimSpace(outputStr) == "" {
		errors.Fatal(
			"No Remote Found",
			"This git repository has no remote configured.\nAdd a remote with: git remote add origin <url>",
			errors.ErrNoRemote,
		)
	}
	return parseRemote(outputStr)
}

// parseRemote extracts owner and repo from git remote output
func parseRemote(remoteOutput string) (*RepoInfo, error) {
	// Regex patterns for both SSH and HTTPS URLs
	// SSH: git@git.com:owner/repo.git
	sshPattern := regexp.MustCompile(`git@github\.com:([^/]+)/([^/\s]+?)(?:\.git)?(?:\s|$)`)
	// HTTPS: https://github.com/owner/repo.git
	httpsPattern := regexp.MustCompile(`https://github\.com/([^/]+)/([^/\s]+?)(?:\.git)?(?:\s|$)`)

	// Try SSH pattern first
	if matches := sshPattern.FindStringSubmatch(remoteOutput); len(matches) >= 3 {
		return &RepoInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	// Try HTTPS pattern
	if matches := httpsPattern.FindStringSubmatch(remoteOutput); len(matches) >= 3 {
		return &RepoInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	errors.Fatal(
		"Invalid Remote URL",
		"Could not parse GitHub repository information from remote.\nOnly GitHub repositories are supported.",
		errors.ErrInvalidRemote,
	)

	return nil, nil // unreachable, but needed for compiler
}

func IsClean() error {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to check git status: %w", err)
	}

	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("the working tree has uncommitted changes. Please commit or stash them")
	}

	return nil
}

func OnMainBranch() error {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unable to determine current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch != "main" && branch != "master" {
		return fmt.Errorf("you are on branch '%s'. Releases are only allowed from 'main' or 'master'", branch)
	}

	return nil
}
