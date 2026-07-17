package main

import (
	"os"
	"strings"
	"testing"
)

func TestDX2MainCurrentlyOwnsOneRequestWorkspaceBoundary(t *testing.T) {
	sourceBytes, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	source := string(sourceBytes)

	workspaceBoundary := strings.Index(source, "workspace.ChangeToProjectRoot(req.Context.WorkingDir)")
	routingBoundary := strings.Index(source, "switch req.Command")
	if workspaceBoundary < 0 {
		t.Fatal("main.go no longer resolves the workspace before command routing")
	}
	if routingBoundary < 0 {
		t.Fatal("main.go command routing boundary not found")
	}
	if workspaceBoundary > routingBoundary {
		t.Fatal("workspace boundary must run before command routing")
	}
	if strings.Contains(source[workspaceBoundary:routingBoundary], "defer ") {
		t.Fatal("current CLI lifecycle should not install a temporary cwd restoration between workspace selection and routing")
	}
}
