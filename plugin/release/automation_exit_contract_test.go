package main

import (
	"os"
	"strings"
	"testing"
)

func TestRepositoryAutomationReliesOnProcessExitWithoutJSONParsing(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"../../.github/workflows/release-neko-cli.yml",
		"../../.github/workflows/release-plugin-release.yml",
		"../../.github/workflows/release-plugin-ui.yml",
	} {
		content := readAutomationExitContractFile(t, path)
		validation := strings.Index(content, "neko release ci-validate-context")
		tests := strings.Index(content, "go test ./...")
		if !strings.Contains(content, "set -euo pipefail") || validation < 0 || tests <= validation {
			t.Fatalf("%s does not stop after CI Context Validation failure", path)
		}
		if strings.Contains(content, "ci-validate-context |") || strings.Contains(content, "ci-validate-context >") {
			t.Fatalf("%s masks CI Context Validation behind output parsing", path)
		}
	}

	makefile := readAutomationExitContractFile(t, "../../Makefile")
	for _, command := range []string{
		"./neko release patch --describe --verbose",
		"./neko release minor --describe --verbose",
		"./neko release major --describe --verbose",
		"./neko release validate",
		"./neko release history",
		"./neko release contributors",
	} {
		if strings.Count(makefile, command) != 1 {
			t.Fatalf("Make lifecycle contract for %q changed", command)
		}
	}

	generator := readAutomationExitContractFile(t, "../../.github/scripts/generate-plugin-index.sh")
	command := strings.Index(generator, "go run . release plugin-index")
	completion := strings.Index(generator, "Generated and validated plugin-index.json")
	if !strings.Contains(generator, "set -euo pipefail") || command < 0 || completion <= command {
		t.Fatal("Plugin Index generation no longer stops before completion messaging")
	}
	if strings.Contains(generator, "go run . release plugin-index |") {
		t.Fatal("Plugin Index generation masks the command exit behind a pipeline")
	}
}

func readAutomationExitContractFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
