package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestLatestUnitTagIgnoresForeignAndInvalidTags(t *testing.T) {
	withGitRepo(t)
	commitFile(t, "api/main.go", "api 1")
	gitCmd(t, "tag", "api/v0.1.0")
	commitFile(t, "web/main.js", "web 1")
	gitCmd(t, "tag", "web/v9.9.9")
	commitFile(t, "api/main.go", "api 2")
	gitCmd(t, "tag", "api/vfoo")
	gitCmd(t, "tag", "api/v0.2.0")

	spec, err := releaseconfig.NewTagSpec("api/v")
	if err != nil {
		t.Fatalf("NewTagSpec: %v", err)
	}
	tag, err := LatestUnitTag(spec)
	if err != nil {
		t.Fatalf("LatestUnitTag: %v", err)
	}
	if tag == nil || tag.Tag != "api/v0.2.0" || tag.Version != "0.2.0" {
		t.Fatalf("unexpected latest tag: %#v", tag)
	}
}

func TestUnitTagsInHistoryAndPathCommitCounts(t *testing.T) {
	withGitRepo(t)
	commitFile(t, "api/main.go", "api 1")
	gitCmd(t, "tag", "api/v0.1.0")
	commitFile(t, "web/main.js", "web 1")
	gitCmd(t, "tag", "web/v0.1.0")
	commitFile(t, "api/main.go", "api 2")
	gitCmd(t, "tag", "api/v0.2.0")

	spec, err := releaseconfig.NewTagSpec("api/v")
	if err != nil {
		t.Fatalf("NewTagSpec: %v", err)
	}
	tags, err := UnitTagsInHistory(spec)
	if err != nil {
		t.Fatalf("UnitTagsInHistory: %v", err)
	}
	if len(tags) != 2 || tags[0].Tag != "api/v0.1.0" || tags[1].Tag != "api/v0.2.0" {
		t.Fatalf("unexpected tags: %#v", tags)
	}

	count, err := CountCommitsBetweenPaths(tags[0].Tag, tags[1].Tag, []string{"api/**"})
	if err != nil {
		t.Fatalf("CountCommitsBetweenPaths: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected only one api commit between tags, got %d", count)
	}
}

func TestContributorsForPaths(t *testing.T) {
	withGitRepo(t)
	commitFile(t, "api/main.go", "api 1")
	commitFile(t, "web/main.js", "web 1")

	contributors, err := ContributorsForPaths([]string{"api/**"})
	if err != nil {
		t.Fatalf("ContributorsForPaths: %v", err)
	}
	if len(contributors) != 1 || contributors[0].Commits != "1" {
		t.Fatalf("expected one api commit, got %#v", contributors)
	}
}

func withGitRepo(t *testing.T) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	gitCmd(t, "init")
	gitCmd(t, "config", "user.email", "test@example.com")
	gitCmd(t, "config", "user.name", "Test User")
}

func commitFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	gitCmd(t, "add", path)
	gitCmd(t, "commit", "-m", "change "+path)
}

func gitCmd(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %s", args, string(out))
	}
}
