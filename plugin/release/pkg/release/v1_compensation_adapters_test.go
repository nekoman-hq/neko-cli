package release

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemV1CompensationConfigRestoresExactOriginalBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".release.neko.json")
	original := []byte("{\n  \"version\": \"1.2.3\"\n}\n")
	if err := os.WriteFile(path, []byte(`{"version":"1.2.4"}`), 0644); err != nil {
		t.Fatalf("write changed config: %v", err)
	}
	files := systemV1CompensationConfigFiles{}

	if err := files.Restore(path, original); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(data) != string(original) {
		t.Fatalf("restored bytes = %q, want %q", data, original)
	}
}

func TestSystemV1CompensationGitDoesNotRepeatAbsentLocalTagDeletion(t *testing.T) {
	runner := &scriptedV1CompensationGitRunner{outputs: []scriptedV1GitOutput{
		{command: "tag --list v1.2.4", output: ""},
	}}
	git := systemV1CompensationGit{effects: systemV1RollbackGit{runner: runner}, runner: runner}

	if err := git.DeleteLocalTag("/repo", "v1.2.4"); err != nil {
		t.Fatalf("DeleteLocalTag: %v", err)
	}
	if got := strings.Join(runner.commands, "\n"); got != "tag --list v1.2.4" {
		t.Fatalf("commands = %q", got)
	}
}

func TestSystemV1CompensationGitVerifiesRemoteTagDeletion(t *testing.T) {
	runner := &scriptedV1CompensationGitRunner{outputs: []scriptedV1GitOutput{
		{command: "push origin --delete v1.2.4"},
		{command: "ls-remote --tags origin refs/tags/v1.2.4", output: "abc refs/tags/v1.2.4\n"},
	}}
	git := systemV1CompensationGit{effects: systemV1RollbackGit{runner: runner}, runner: runner}

	err := git.DeleteRemoteTag("/repo", "v1.2.4")

	if err == nil || !strings.Contains(err.Error(), "tag v1.2.4 is still present") {
		t.Fatalf("DeleteRemoteTag error = %v", err)
	}
}

type scriptedV1GitOutput struct {
	err     error
	command string
	output  string
}

type scriptedV1CompensationGitRunner struct {
	outputs  []scriptedV1GitOutput
	commands []string
}

func (runner *scriptedV1CompensationGitRunner) CombinedOutput(_ string, args ...string) ([]byte, error) {
	command := strings.Join(args, " ")
	runner.commands = append(runner.commands, command)
	if len(runner.outputs) == 0 {
		return nil, errors.New("unexpected git command")
	}
	output := runner.outputs[0]
	runner.outputs = runner.outputs[1:]
	if output.command != command {
		return nil, errors.New("unexpected git command order")
	}
	return []byte(output.output), output.err
}
