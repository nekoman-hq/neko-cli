package release

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

const githubAPIVersion = "2026-03-10"

func githubActionsDispatchUserAgent() string {
	version := strings.TrimSpace(metadata.Version)
	if version == "" {
		version = "dev"
	}
	return "neko-cli/" + version
}

// GitHubActionsDispatchToken is a validated secret-bearing value. Its string
// representations are always redacted; only authenticated GitHub adapters can
// unwrap it.
type GitHubActionsDispatchToken struct {
	secret string
}

// NewGitHubActionsDispatchToken validates a token for explicit resolver and
// client implementations without exposing its value through formatting.
func NewGitHubActionsDispatchToken(value string) (GitHubActionsDispatchToken, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return GitHubActionsDispatchToken{}, missingGitHubActionsDispatchTokenError()
	}
	return GitHubActionsDispatchToken{secret: value}, nil
}

func (token GitHubActionsDispatchToken) secretValue() string {
	return token.secret
}

func (GitHubActionsDispatchToken) String() string {
	return "[redacted]"
}

func (GitHubActionsDispatchToken) GoString() string {
	return "[redacted]"
}

// GitHubActionsDispatchTokenResolver resolves the production dispatch token.
type GitHubActionsDispatchTokenResolver interface {
	ResolveGitHubActionsDispatchToken(ctx context.Context) (GitHubActionsDispatchToken, error)
}

// EnvironmentGitHubActionsDispatchTokenResolver resolves GITHUB_TOKEN for real
// authenticated GitHub operations.
type EnvironmentGitHubActionsDispatchTokenResolver struct{}

func (EnvironmentGitHubActionsDispatchTokenResolver) ResolveGitHubActionsDispatchToken(_ context.Context) (GitHubActionsDispatchToken, error) {
	return NewGitHubActionsDispatchToken(os.Getenv("GITHUB_TOKEN"))
}

func missingGitHubActionsDispatchTokenError() error {
	return fmt.Errorf("GitHub Actions dispatch requires GITHUB_TOKEN with the appropriate repository Actions write permission")
}
