package releaseworkflow

import "testing"

func TestResolveGitHubRepositoryTargetSupportsCanonicalRemoteForms(t *testing.T) {
	for _, remoteURL := range []string{
		"https://github.com/OWNER/REPOSITORY.git",
		"https://github.com/OWNER/REPOSITORY",
		"ssh://git@github.com/OWNER/REPOSITORY.git",
		"ssh://git@github.com/OWNER/REPOSITORY",
		"git@github.com:OWNER/REPOSITORY.git",
		"git@github.com:OWNER/REPOSITORY",
	} {
		t.Run(remoteURL, func(t *testing.T) {
			target, err := ResolveGitHubRepositoryTarget("origin", remoteURL)
			if err != nil {
				t.Fatalf("ResolveGitHubRepositoryTarget: %v", err)
			}
			if target.Owner != "OWNER" || target.Repository != "REPOSITORY" ||
				target.RemoteName != "origin" || target.CanonicalRemoteURL != "https://github.com/OWNER/REPOSITORY" ||
				target.APIBaseURL != "https://api.github.com" {
				t.Fatalf("unexpected target: %#v", target)
			}
		})
	}
}

func TestResolveGitHubRepositoryTargetRejectsUnsupportedRemotes(t *testing.T) {
	for _, remoteURL := range []string{
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
	} {
		t.Run(remoteURL, func(t *testing.T) {
			if _, err := ResolveGitHubRepositoryTarget("origin", remoteURL); err == nil {
				t.Fatal("expected unsupported remote rejection")
			}
		})
	}
}

func TestSanitizeRemoteForLogRemovesURLCredentials(t *testing.T) {
	if got := SanitizeRemoteForLog("https://token@example.test/owner/repo.git"); got != "https://example.test/owner/repo.git" {
		t.Fatalf("sanitized remote = %q", got)
	}
	for _, unchanged := range []string{
		"https://github.com/owner/repo.git",
		"git@github.com:owner/repo.git",
		"https://%zz",
	} {
		if got := SanitizeRemoteForLog(unchanged); got != unchanged {
			t.Fatalf("credential-free or malformed remote changed: %q -> %q", unchanged, got)
		}
	}
}
