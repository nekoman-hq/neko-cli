package releaseworkflow

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

// GitHubActionsAPIBaseURL is the canonical GitHub.com REST API origin used by
// dispatch and explicit read-only verification adapters.
const GitHubActionsAPIBaseURL = "https://api.github.com"

var githubRepositorySegmentRegexp = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// GitHubRepositoryTarget identifies the GitHub.com repository that receives a
// workflow_dispatch request.
type GitHubRepositoryTarget struct {
	Owner              string
	Repository         string
	RemoteName         string
	CanonicalRemoteURL string
	APIBaseURL         string
}

// ResolveGitHubRepositoryTarget derives the dispatch target from the verified
// Git remote selected by V2 Git coordination.
func ResolveGitHubRepositoryTarget(remoteName, remoteURL string) (GitHubRepositoryTarget, error) {
	remoteName = strings.TrimSpace(remoteName)
	remoteURL = strings.TrimSpace(remoteURL)
	if remoteName == "" {
		return GitHubRepositoryTarget{}, fmt.Errorf("github actions dispatch target requires the selected git remote name")
	}
	if remoteURL == "" {
		return GitHubRepositoryTarget{}, fmt.Errorf("github actions dispatch target requires the selected git remote URL")
	}
	if containsWhitespace(remoteURL) {
		return GitHubRepositoryTarget{}, fmt.Errorf("github actions dispatch remote URL contains whitespace")
	}
	if strings.ContainsAny(remoteURL, "`$|;&<>") {
		return GitHubRepositoryTarget{}, fmt.Errorf("github actions dispatch remote URL contains unsupported shell-like syntax")
	}
	var owner, repository string
	var err error
	switch {
	case strings.HasPrefix(remoteURL, "git@github.com:"):
		owner, repository, err = parseGitHubSCPRemote(remoteURL)
	default:
		owner, repository, err = parseGitHubURLRemote(remoteURL)
	}
	if err != nil {
		return GitHubRepositoryTarget{}, err
	}
	return GitHubRepositoryTarget{
		Owner:              owner,
		Repository:         repository,
		RemoteName:         remoteName,
		CanonicalRemoteURL: fmt.Sprintf("https://github.com/%s/%s", owner, repository),
		APIBaseURL:         GitHubActionsAPIBaseURL,
	}, nil
}

func parseGitHubSCPRemote(remoteURL string) (string, string, error) {
	if strings.ContainsAny(remoteURL, "?#") {
		return "", "", fmt.Errorf("github actions dispatch remote URL must not include query strings or fragments")
	}
	path := strings.TrimPrefix(remoteURL, "git@github.com:")
	if strings.Contains(path, ":") {
		return "", "", fmt.Errorf("github actions dispatch remote URL contains unsupported credential or port syntax")
	}
	return parseGitHubOwnerRepository(path)
}

func parseGitHubURLRemote(remoteURL string) (string, string, error) {
	parsed, err := url.Parse(remoteURL)
	if err != nil {
		return "", "", fmt.Errorf("parse github actions dispatch remote URL: %w", err)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", "", fmt.Errorf("github actions dispatch remote URL must not include query strings or fragments")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "github.com" {
		if strings.Contains(host, "github") && host != "" {
			return "", "", fmt.Errorf("GitHub Actions dispatch currently supports GitHub.com remotes only; unsupported host %q", parsed.Hostname())
		}
		return "", "", fmt.Errorf("GitHub Actions dispatch currently supports GitHub.com remotes only; unsupported host %q", parsed.Hostname())
	}
	if parsed.Port() != "" {
		return "", "", fmt.Errorf("github actions dispatch remote URL must not include a port")
	}
	switch parsed.Scheme {
	case "https":
		if parsed.User != nil {
			return "", "", fmt.Errorf("github actions dispatch HTTPS remote URL must not include credentials")
		}
		return parseGitHubOwnerRepository(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	case "ssh":
		if parsed.User == nil || parsed.User.Username() != "git" {
			return "", "", fmt.Errorf("github actions dispatch SSH remote must use git@github.com")
		}
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return "", "", fmt.Errorf("github actions dispatch SSH remote must not include credentials beyond git@github.com")
		}
		return parseGitHubOwnerRepository(strings.TrimPrefix(parsed.EscapedPath(), "/"))
	default:
		return "", "", fmt.Errorf("github actions dispatch supports https and git SSH GitHub.com remotes only")
	}
}

func parseGitHubOwnerRepository(path string) (string, string, error) {
	if strings.Contains(path, "%") {
		return "", "", fmt.Errorf("github actions dispatch remote path must not contain escaped or encoded characters")
	}
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("github actions dispatch remote path must be exactly OWNER/REPOSITORY")
	}
	owner := parts[0]
	repository := strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repository == "" {
		return "", "", fmt.Errorf("github actions dispatch remote owner and repository must not be empty")
	}
	if owner == "." || owner == ".." || repository == "." || repository == ".." {
		return "", "", fmt.Errorf("github actions dispatch remote owner and repository must not use traversal")
	}
	if strings.Contains(owner, "..") || strings.Contains(repository, "..") {
		return "", "", fmt.Errorf("github actions dispatch remote owner and repository must not use traversal")
	}
	if !githubRepositorySegmentRegexp.MatchString(owner) || !githubRepositorySegmentRegexp.MatchString(repository) {
		return "", "", fmt.Errorf("github actions dispatch remote owner or repository contains unsupported characters")
	}
	return owner, repository, nil
}

func containsWhitespace(value string) bool {
	return strings.IndexFunc(value, unicode.IsSpace) >= 0
}

// SanitizeRemoteForLog removes URL credentials before a Git remote identity is
// emitted as progress text. Non-URL and credential-free forms are unchanged.
func SanitizeRemoteForLog(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User == nil {
		return raw
	}
	parsed.User = nil
	return parsed.String()
}
