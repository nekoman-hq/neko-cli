package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestV1ActivePreflightFailureKeepsFatalProcessContract(t *testing.T) {
	if os.Getenv("NEKO_TEST_V1_ACTIVE_FATAL") == "1" {
		main()
		return
	}

	root := t.TempDir()
	v1 := `{"project-name":"example","project-owner":"acme","project-type":"backend","release-system":"goreleaser","version":"1.2.3"}`
	if err := os.WriteFile(filepath.Join(root, ".release.neko.json"), []byte(v1), 0644); err != nil {
		t.Fatalf("write V1 config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write GoReleaser config: %v", err)
	}
	binDir := t.TempDir()
	gitScript := `#!/bin/sh
case "$*" in
  "describe --tags --abbrev=0") printf 'v1.2.3\n' ;;
  "remote -v") printf 'origin\thttps://github.com/acme/example.git (fetch)\norigin\thttps://github.com/acme/example.git (push)\n' ;;
  "status --porcelain") printf ' M dirty-file\n' ;;
esac
`
	if err := os.WriteFile(filepath.Join(binDir, "git"), []byte(gitScript), 0755); err != nil {
		t.Fatalf("write fake git: %v", err)
	}

	request, err := json.Marshal(plugin.Request{
		Command: "patch",
		Context: plugin.Context{WorkingDir: root},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestV1ActivePreflightFailureKeepsFatalProcessContract$")
	cmd.Stdin = bytes.NewReader(append(request, '\n'))
	cmd.Env = append(os.Environ(),
		"NEKO_TEST_V1_ACTIVE_FATAL=1",
		"GITHUB_TOKEN=test-token",
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	output, err := cmd.Output()
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("active V1 fatal exit = %v, want code 1; output=%s", err, output)
	}

	var response plugin.Response
	if err := json.Unmarshal(output, &response); err != nil {
		t.Fatalf("decode fatal response %q: %v", output, err)
	}
	if response.Status != "error" || response.Error == nil || response.Error.Code != "UNCOMMITTED_CHANGES" {
		t.Fatalf("unexpected fatal response: %#v", response)
	}
	if response.Metadata.Command != "" {
		t.Fatalf("fatal response command = %q, want empty", response.Metadata.Command)
	}
}
