package release

import (
	"strings"
	"testing"
)

func TestResolveGitHubRepositoryTargetSupportsCanonicalRemoteForms(t *testing.T) {
	tests := []string{
		"https://github.com/OWNER/REPOSITORY.git",
		"https://github.com/OWNER/REPOSITORY",
		"ssh://git@github.com/OWNER/REPOSITORY.git",
		"ssh://git@github.com/OWNER/REPOSITORY",
		"git@github.com:OWNER/REPOSITORY.git",
		"git@github.com:OWNER/REPOSITORY",
	}
	for _, remoteURL := range tests {
		t.Run(remoteURL, func(t *testing.T) {
			target, err := ResolveGitHubRepositoryTarget("origin", remoteURL)
			if err != nil {
				t.Fatalf("ResolveGitHubRepositoryTarget: %v", err)
			}
			if target.Owner != "OWNER" || target.Repository != "REPOSITORY" {
				t.Fatalf("unexpected owner/repository: %#v", target)
			}
			if target.RemoteName != "origin" || target.CanonicalRemoteURL != "https://github.com/OWNER/REPOSITORY" {
				t.Fatalf("unexpected target: %#v", target)
			}
			if target.APIBaseURL != "https://api.github.com" {
				t.Fatalf("unexpected API base URL: %s", target.APIBaseURL)
			}
		})
	}
}

func TestResolveGitHubRepositoryTargetRejectsUnsupportedRemotes(t *testing.T) {
	tests := []string{
		"https://github.example.com/OWNER/REPOSITORY.git",
		"https://gitlab.com/OWNER/REPOSITORY.git",
		"https://bitbucket.org/OWNER/REPOSITORY.git",
		"file:///tmp/repository.git",
		"not a url",
		"https://github.com/OWNER/REPOSITORY/extra.git",
		"https://github.com/OWNER/REPOSITORY.git?x=1",
		"https://github.com/OWNER/REPOSITORY.git#frag",
		"https://github.com/OWNER/../REPOSITORY.git",
		"https://github.com/OWNER/REPO SITORY.git",
		"https://token@github.com/OWNER/REPOSITORY.git",
		"ssh://root@github.com/OWNER/REPOSITORY.git",
		"git@github.example.com:OWNER/REPOSITORY.git",
	}
	for _, remoteURL := range tests {
		t.Run(remoteURL, func(t *testing.T) {
			if _, err := ResolveGitHubRepositoryTarget("origin", remoteURL); err == nil {
				t.Fatal("expected unsupported remote rejection")
			}
		})
	}
}

func TestDispatchTargetUsesExactSelectedV2ReleaseRemote(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	gitCmd(t, root, "remote", "add", "upstream", "git@github.com:nekoman/selected.git")
	branch := strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
	gitCmd(t, root, "config", "branch."+branch+".remote", "upstream")
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request := mustBuildDispatchRequest(t, ctx, result)

	target, err := ResolveGitHubRepositoryTarget(request.RepositoryRemoteName, request.Identity.RepositoryRemote)
	if err != nil {
		t.Fatalf("ResolveGitHubRepositoryTarget: %v", err)
	}
	if target.RemoteName != "upstream" || target.Owner != "nekoman" || target.Repository != "selected" {
		t.Fatalf("dispatch target did not use selected V2 remote: %#v", target)
	}
}
