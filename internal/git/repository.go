// Package git includes operations using git or git-cli
package git

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since     17.12.2025
*/

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/nekoman-hq/neko-cli/internal/log"
)

type RepoInfo struct {
	Owner string
	Repo  string
}

type Contributor struct {
	Commits string
	Author  string
}

func Fetch() {
	log.V(log.VersionGuard, fmt.Sprintf("%s (Updating repository information)",
		log.ColorText(log.ColorGreen, "git fetch"),
	))

	_ = exec.Command("git", "fetch").Run()
}

// Current checks if a git repository exists and returns owner and repo name
func Current() (*RepoInfo, error) {
	log.V(log.Config, fmt.Sprintf("%s (Checking Repository Origin)",
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
func parseRemote(remoteOutput string) (*RepoInfo, error) {
	// Regex patterns for both SSH and HTTPS URLs
	// SSH: git@git.com:owner/repo.git
	sshPattern := regexp.MustCompile(`git@github\.com:([^/]+)/([^/\s]+?)(?:\.git)?(?:\s|$)`)
	// HTTPS: https://github.com/owner/repo.git
	httpsPattern := regexp.MustCompile(`https://github\.com/([^/]+)/([^/\s]+?)(?:\.git)?(?:\s|$)`)

	// Try SSH pattern first
	if matches := sshPattern.FindStringSubmatch(remoteOutput); len(matches) >= 3 {
		repoPath := fmt.Sprintf("%s/%s", matches[1], matches[2])
		log.V(log.Config, fmt.Sprintf("Found repository: %s (SSH)",
			log.ColorText(log.ColorGreen, repoPath)))
		return &RepoInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	// Try HTTPS pattern
	if matches := httpsPattern.FindStringSubmatch(remoteOutput); len(matches) >= 3 {
		repoPath := fmt.Sprintf("%s/%s", matches[1], matches[2])
		log.V(log.Config, fmt.Sprintf("Found repository: %s (HTTPS)",
			log.ColorText(log.ColorGreen, repoPath)))
		return &RepoInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	return nil, errors.New(
		"invalid Remote URL: Could not parse GitHub repository information from remote.\nOnly GitHub repositories are supported",
	)
}

func IsClean() error {
	log.V(log.Preflight, fmt.Sprintf("%s (Check branch state)",
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

	log.V(log.Preflight, "Working tree is clean")
	return nil
}

func EnsureNotDetached() error {
	log.V(log.Preflight, fmt.Sprintf("%s (Ensure branch is not detached)",
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

	log.V(log.Preflight, fmt.Sprintf("HEAD attached to branch %s", log.ColorText(log.ColorGreen, branch)))
	return nil
}

func OnMainBranch() error {
	log.V(log.Preflight, fmt.Sprintf("%s (Check on main branch)",
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

	log.V(log.Preflight, fmt.Sprintf("On %s branch", log.ColorText(log.ColorGreen, branch)))
	return nil
}

func HasUpstream() error {
	log.V(log.Preflight, fmt.Sprintf("%s (Check upstream configuration)",
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

	log.V(log.Preflight, fmt.Sprintf("Upstream branch: %s", log.ColorText(log.ColorGreen, upstream)))
	return nil
}

func IsUpToDate() error {
	log.V(log.Preflight, fmt.Sprintf("%s (Check if branch is up to date)",
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

	log.V(log.Preflight, "Branch is up to date with upstream")
	return nil
}

// CurrentBranch returns the name of the current branch
func CurrentBranch() (string, error) {
	log.V(log.History, "Fetching current branch: "+
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

// LastCommit returns the last commit information
func LastCommit() (string, error) {
	log.V(log.History, "Fetching last commit: "+
		log.ColorText(log.ColorGreen, "git log -1 --pretty=format:%%h '%%s' (%%cr)"))

	cmd := exec.Command("git", "log", "-1", "--pretty=format:%h '%s' (%cr)")
	lastCommitOut, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(
			"failed to get last commit: %w", err,
		)
	}

	lastCommit := strings.TrimSpace(string(lastCommitOut))
	return lastCommit, nil
}

// TotalCommits returns the total number of commits as a string
func TotalCommits() (string, error) {
	log.V(log.History, "Counting total commits: "+
		log.ColorText(log.ColorGreen, "git rev-list --count HEAD"))

	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	totalCommitsOut, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf(
			"failed to count commits: %w", err,
		)
	}

	return strings.TrimSpace(string(totalCommitsOut)), nil
}

// FilesCount returns the number of tracked files
func FilesCount() (int, error) {
	log.V(log.History, "Counting tracked files: "+
		log.ColorText(log.ColorGreen, "git ls-files"))

	cmd := exec.Command("git", "ls-files")
	filesOut, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf(
			"failed to count files: %w", err,
		)
	}

	files := strings.Split(strings.TrimSpace(string(filesOut)), "\n")
	return len(files), nil
}

// RepoSize returns the repository size using du command
func RepoSize() (string, error) {
	log.V(log.History, "Calculating repository size: "+
		log.ColorText(log.ColorGreen, "du -sh ."))

	cmd := exec.Command("du", "-sh", ".")
	sizeOut, err := cmd.Output()
	if err != nil {
		return "", errors.New("could not determine repository size (du command not available")
	}

	fields := strings.Fields(string(sizeOut))
	if len(fields) == 0 {
		return "", errors.New("failed determing repository size")
	}

	return fields[0], nil
}

// Contributors returns a list of contributors with their commit counts
func Contributors() ([]Contributor, error) {
	log.V(log.History, "Fetching contributors: "+
		log.ColorText(log.ColorGreen, "git shortlog -sne HEAD"))

	cmd := exec.Command("git", "shortlog", "-sne", "HEAD")
	contrib, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch contributors: %w", err,
		)
	}

	contribLines := strings.Split(strings.TrimSpace(string(contrib)), "\n")
	log.V(log.History, fmt.Sprintf("Found %d contributors", len(contribLines)))

	contributors := make([]Contributor, 0, len(contribLines))
	for _, line := range contribLines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			log.V(log.History, fmt.Sprintf("Skipping invalid contributor line: %s", line))
			continue
		}

		contributors = append(contributors, Contributor{
			Commits: parts[0],
			Author:  strings.Join(parts[1:], " "),
		})
	}

	return contributors, nil
}

func DeleteGithubRelease(tag string, token string) error {
	if tag == "" {
		return nil
	}
	if token == "" {
		return fmt.Errorf("github token is empty")
	}

	repo, err := Current()
	if err != nil {
		return err
	}

	owner := repo.Owner
	name := repo.Repo

	// Resolve release by tag -> get release id
	getURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, name, tag)

	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "neko-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// If release does not exist, rollback should be idempotent -> success.
	if resp.StatusCode == http.StatusNotFound {
		log.V(log.Release, fmt.Sprintf("GitHub release for tag %s not found (nothing to delete)", tag))
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github: failed fetching release by tag %s: status=%d body=%s", tag, resp.StatusCode, string(body))
	}

	var payload struct {
		ID int64 `json:"id"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	if payload.ID == 0 {
		return fmt.Errorf("github: release id missing for tag %s", tag)
	}

	// Delete release by id
	delURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/%d", owner, name, payload.ID)

	delReq, err := http.NewRequest("DELETE", delURL, nil)
	if err != nil {
		return err
	}
	delReq.Header.Set("Authorization", "Bearer "+token)
	delReq.Header.Set("Accept", "application/vnd.github+json")
	delReq.Header.Set("User-Agent", "neko-cli")

	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		return err
	}
	defer func() { _ = delResp.Body.Close() }()

	if delResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(delResp.Body)
		return fmt.Errorf("github: failed deleting release for tag %s: status=%d body=%s", tag, delResp.StatusCode, string(body))
	}

	log.V(log.Release, fmt.Sprintf("Deleted GitHub release for tag %s", tag))
	return nil
}

// Head returns the current commit hash of HEAD.
func Head() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD failed: %s", strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// CleanUntracked removes untracked files and directories.
func CleanUntracked() error {
	cmd := exec.Command("git", "clean", "-fd")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clean -fd failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteLocalTag deletes a local git tag.
func DeleteLocalTag(tag string) error {
	cmd := exec.Command("git", "tag", "-d", tag)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git tag -d %s failed: %s", tag, strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteRemoteTag deletes a tag from origin.
func DeleteRemoteTag(tag string) error {
	cmd := exec.Command("git", "push", "origin", "--delete", tag)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push origin --delete %s failed: %s", tag, strings.TrimSpace(string(out)))
	}
	return nil
}

// RevertCommit creates a new commit that reverts the given commit hash.
func RevertCommit(hash string) error {
	cmd := exec.Command("git", "revert", "--no-edit", hash)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git revert %s failed: %s", hash, strings.TrimSpace(string(out)))
	}
	return nil
}

// CreateCommit creates a new commit with a given message
func CreateCommit(message string) error {
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", message)
	out, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Errorf("git commit -m '%s' failed: %s", message, strings.TrimSpace(string(out)))
	}

	return nil
}

// HardResetTo resets HEAD, index, and working tree to the given commit hash.
func HardResetTo(hash string) error {
	cmd := exec.Command("git", "reset", "--hard", hash)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --hard %s failed: %s", hash, strings.TrimSpace(string(out)))
	}
	return nil
}
