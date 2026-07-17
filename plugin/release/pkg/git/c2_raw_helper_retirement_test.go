package git

import (
	"os"
	"strings"
	"testing"
)

func TestC2RawGitHelpersStayRemoved(t *testing.T) {
	data, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	source := string(data)
	for _, signature := range []string{
		"func " + "LastCommit(",
		"func " + "TotalCommits(",
		"func " + "FilesCount(",
		"func " + "RepoSize(",
		"func " + "DeleteGithubRelease(",
		"func " + "Head(",
		"func " + "CleanUntracked(",
		"func " + "DeleteLocalTag(",
		"func " + "DeleteRemoteTag(",
		"func " + "RevertCommit(",
		"func " + "CreateCommit(",
		"func " + "HardResetTo(",
	} {
		if strings.Contains(source, signature) {
			t.Fatalf("repository.go reintroduced retired raw Git helper %q", signature)
		}
	}
}
