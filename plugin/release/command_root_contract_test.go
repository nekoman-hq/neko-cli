package main

import (
	"os"
	"strings"
	"testing"
)

func TestMainOwnsOneExplicitRootBoundary(t *testing.T) {
	sourceBytes, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	source := string(sourceBytes)

	workspaceBoundary := strings.Index(source, "workspace.ResolveRepositoryRoot(req.Context.WorkingDir)")
	routingBoundary := strings.Index(source, "handleRequestAt(root, req, v1Executors)")
	if workspaceBoundary < 0 {
		t.Fatal("main.go no longer resolves the workspace before command routing")
	}
	if routingBoundary < 0 {
		t.Fatal("main.go command routing boundary not found")
	}
	if workspaceBoundary > routingBoundary {
		t.Fatal("workspace boundary must run before command routing")
	}
	if strings.Contains(source, "workspace.ChangeToProjectRoot(") || strings.Contains(source, "os.Chdir(") {
		t.Fatal("main.go must not mutate process cwd during explicit-root routing")
	}
	for _, required := range []string{
		"initcmd.HandleInitAt(root, req)",
		"initcmd.HandleUnitAddAt(root, req)",
		"release.HandleReleaseWithV1ExecutorsAt(root, req, release.Patch, v1Executors...)",
		"release.HandleResumeAt(root, req)",
		"evidence.HandleEvidenceAt(root, req)",
		"history.HandleHistoryAt(root, req)",
		"contributors.HandleContributorsAt(root, req)",
		"validate.HandleValidateAt(root, req)",
		"pluginindex.HandlePluginIndexAt(root, req)",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("main.go does not route through explicit-root API %q", required)
		}
	}
}
