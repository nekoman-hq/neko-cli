package doctor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

const githubReadAPIVersion = "2026-03-10"

type githubReadToken struct {
	secret string
}

func newGitHubReadToken(value string) (githubReadToken, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return githubReadToken{}, fmt.Errorf("GitHub read token is missing")
	}
	return githubReadToken{secret: value}, nil
}

func (token githubReadToken) secretValue() string {
	return token.secret
}

func (githubReadToken) String() string {
	return "[redacted]"
}

func (githubReadToken) GoString() string {
	return "[redacted]"
}

type githubReadTokenResolver interface {
	ResolveGitHubReadToken(context.Context) (githubReadToken, error)
}

type environmentGitHubReadTokenResolver struct{}

func (environmentGitHubReadTokenResolver) ResolveGitHubReadToken(_ context.Context) (githubReadToken, error) {
	return newGitHubReadToken(os.Getenv("GITHUB_TOKEN"))
}

func githubReadUserAgent() string {
	version := strings.TrimSpace(metadata.Version)
	if version == "" {
		version = "dev"
	}
	return "neko-cli/" + version
}
