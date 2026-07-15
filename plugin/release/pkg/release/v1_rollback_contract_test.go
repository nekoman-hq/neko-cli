package release

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type v1RollbackRoundTripper struct {
	requests   []string
	getCodes   []int
	getCode    int
	getCalls   int
	deleteCode int
}

func (transport *v1RollbackRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests = append(transport.requests, request.Method+" "+request.URL.String())
	status := transport.getCode
	body := `{"id":42}`
	if request.Method == http.MethodGet && transport.getCalls < len(transport.getCodes) {
		status = transport.getCodes[transport.getCalls]
		transport.getCalls++
	}
	if request.Method == http.MethodDelete {
		status = transport.deleteCode
		body = ""
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}, nil
}

func TestV1RollbackCompatibilitySequence(t *testing.T) {
	logPath := installV1FakeGit(t)
	t.Setenv("GITHUB_TOKEN", "test-token")

	transport := &v1RollbackRoundTripper{
		getCodes:   []int{http.StatusOK, http.StatusNotFound},
		deleteCode: http.StatusNoContent,
	}

	rollback := newV1RollbackWithTransport(transport)
	err := rollback.Rollback("", GitReleaseState{
		PreHead:              "before",
		ReleaseHead:          "release",
		TagName:              "v1.2.4",
		GitHubReleaseTag:     "v1.2.4",
		PushedCommit:         true,
		PushedTag:            true,
		CreatedGitHubRelease: true,
	})
	if err != nil {
		t.Fatalf("RevertGitRelease: %v", err)
	}

	wantHTTP := []string{
		"GET https://api.github.com/repos/acme/example/releases/tags/v1.2.4",
		"DELETE https://api.github.com/repos/acme/example/releases/42",
		"GET https://api.github.com/repos/acme/example/releases/tags/v1.2.4",
	}
	if got := strings.Join(transport.requests, "\n"); got != strings.Join(wantHTTP, "\n") {
		t.Fatalf("GitHub rollback sequence:\n%s\nwant:\n%s", got, strings.Join(wantHTTP, "\n"))
	}
	wantGit := []string{
		"tag -d v1.2.4",
		"push origin --delete v1.2.4",
		"revert --no-edit release",
		"push origin HEAD",
		"clean -fd",
	}
	assertV1CommandLog(t, logPath, wantGit)
}

func TestV1RollbackRevertFailureUsesFallbackCommit(t *testing.T) {
	logPath := installV1FakeGit(t)
	t.Setenv("NEKO_V1_GIT_FAIL", "revert --no-edit release")

	var tool ToolBase
	if err := tool.RevertGitRelease(GitReleaseState{
		ReleaseHead:  "release",
		PushedCommit: true,
	}); err != nil {
		t.Fatalf("RevertGitRelease: %v", err)
	}
	assertV1CommandLog(t, logPath, []string{
		"revert --no-edit release",
		"commit --allow-empty -m revert release",
		"push origin HEAD",
		"clean -fd",
	})
}

func TestV1RollbackWithoutPushedCommitHardResetsThenCleans(t *testing.T) {
	logPath := installV1FakeGit(t)

	var tool ToolBase
	if err := tool.RevertGitRelease(GitReleaseState{PreHead: "before", ReleaseHead: "release"}); err != nil {
		t.Fatalf("RevertGitRelease: %v", err)
	}
	assertV1CommandLog(t, logPath, []string{"reset --hard before", "clean -fd"})
}

func TestV1RollbackStopsAtFirstSurfacedFailure(t *testing.T) {
	tests := []struct { //nolint:govet // Logical failure-matrix order keeps state, trigger, and observations adjacent.
		name      string
		state     GitReleaseState
		fail      string
		wantLog   []string
		wantError string
	}{
		{
			name:      "remote tag deletion",
			state:     GitReleaseState{TagName: "v1.2.4", PushedTag: true},
			fail:      "push origin --delete v1.2.4",
			wantLog:   []string{"tag -d v1.2.4", "push origin --delete v1.2.4"},
			wantError: "rollback: failed deleting remote tag v1.2.4",
		},
		{
			name:      "hard reset",
			state:     GitReleaseState{PreHead: "before", ReleaseHead: "release"},
			fail:      "reset --hard before",
			wantLog:   []string{"reset --hard before"},
			wantError: "rollback: failed hard reset to before",
		},
		{
			name:      "push revert",
			state:     GitReleaseState{ReleaseHead: "release", PushedCommit: true},
			fail:      "push origin HEAD",
			wantLog:   []string{"revert --no-edit release", "push origin HEAD"},
			wantError: "rollback: failed pushing revert commit",
		},
		{
			name:      "cleanup",
			state:     GitReleaseState{UpdatedConfig: true},
			fail:      "clean -fd",
			wantLog:   []string{"clean -fd"},
			wantError: "rollback: failed cleaning untracked files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := installV1FakeGit(t)
			t.Setenv("NEKO_V1_GIT_FAIL", tt.fail)
			var tool ToolBase
			err := tool.RevertGitRelease(tt.state)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("RevertGitRelease error = %v, want containing %q", err, tt.wantError)
			}
			assertV1CommandLog(t, logPath, tt.wantLog)
		})
	}
}

func TestV1RollbackRejectsInconsistentCommitStateBeforeCleanup(t *testing.T) {
	logPath := installV1FakeGit(t)
	var tool ToolBase
	err := tool.RevertGitRelease(GitReleaseState{ReleaseHead: "release"})
	if err == nil || err.Error() != "rollback: inconsistent state (release commit exists but pre-head missing)" {
		t.Fatalf("RevertGitRelease error = %v", err)
	}
	assertV1CommandLog(t, logPath, nil)
}

func installV1FakeGit(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git.log")
	script := `#!/bin/sh
printf '%s\n' "$*" >> "$NEKO_V1_GIT_LOG"
if [ "$*" = "remote -v" ]; then
  printf 'origin\thttps://github.com/acme/example.git (fetch)\norigin\thttps://github.com/acme/example.git (push)\n'
fi
if [ -n "$NEKO_V1_GIT_FAIL" ] && [ "$*" = "$NEKO_V1_GIT_FAIL" ]; then
  printf 'injected git failure\n' >&2
  exit 23
fi
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(script), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}
	t.Setenv("NEKO_V1_GIT_LOG", logPath)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func assertV1CommandLog(t *testing.T, path string, want []string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && len(want) == 0 {
			return
		}
		t.Fatalf("read command log: %v", err)
	}
	gotText := strings.TrimSpace(string(data))
	got := []string(nil)
	if gotText != "" {
		got = strings.Split(gotText, "\n")
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("command log:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestV1RollbackHTTPFailurePrecedesGitDestruction(t *testing.T) {
	logPath := installV1FakeGit(t)
	t.Setenv("GITHUB_TOKEN", "test-token")
	transport := &v1RollbackRoundTripper{getCode: http.StatusInternalServerError, deleteCode: http.StatusNoContent}

	rollback := newV1RollbackWithTransport(transport)
	err := rollback.Rollback("", GitReleaseState{
		TagName:              "v1.2.4",
		GitHubReleaseTag:     "v1.2.4",
		PushedTag:            true,
		CreatedGitHubRelease: true,
	})
	if err == nil || !strings.Contains(err.Error(), "rollback: failed deleting GitHub release v1.2.4") {
		t.Fatalf("RevertGitRelease error = %v", err)
	}
	assertV1CommandLog(t, logPath, nil)
}

func newV1RollbackWithTransport(transport http.RoundTripper) *V1ReleaseRollback {
	runner := newSystemV1GitCommandRunner()
	return &V1ReleaseRollback{
		git: systemV1RollbackGit{runner: runner},
		releases: systemV1GitHubReleaseRemover{
			tokens: fixedV1TokenResolver{token: "test-token"},
			client: boundedV1GitHubReleaseClient{
				http:    &http.Client{Transport: transport},
				remotes: fixedV1GitHubRemoteURLReader{remoteURL: "https://github.com/acme/example.git"},
			},
		},
	}
}

type fixedV1GitHubRemoteURLReader struct {
	remoteURL string
}

func (reader fixedV1GitHubRemoteURLReader) ReadOriginURL(string) (string, error) {
	return reader.remoteURL, nil
}
