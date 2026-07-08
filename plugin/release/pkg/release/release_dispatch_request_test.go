package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestBuildReleaseDispatchRequestCreatesDeterministicRequest(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)

	request, err := BuildReleaseDispatchRequest(ctx, result)
	if err != nil {
		t.Fatalf("BuildReleaseDispatchRequest: %v", err)
	}
	if request.UnitID != "api" || request.Version != "0.2.1" || request.Tag != "api/v0.2.1" {
		t.Fatalf("unexpected request: %#v", request)
	}
	if request.WorkflowPath != ".github/workflows/release-api.yml" || request.WorkflowFileName != "release-api.yml" {
		t.Fatalf("unexpected workflow fields: %#v", request)
	}
	if request.RepositoryRemoteName != "origin" || request.Identity.RepositoryRemoteName != "origin" {
		t.Fatalf("unexpected remote name fields: %#v", request)
	}
	if keys := sortedDispatchInputKeys(request.Inputs); strings.Join(keys, ",") != "release_sha,tag,unit,version" {
		t.Fatalf("unexpected input keys: %#v", keys)
	}
	if request.Inputs["release_sha"] != result.CommitSHA || request.Inputs["unit"] != "api" {
		t.Fatalf("unexpected inputs: %#v", request.Inputs)
	}

	again, err := BuildReleaseDispatchRequest(ctx, result)
	if err != nil {
		t.Fatalf("BuildReleaseDispatchRequest again: %v", err)
	}
	if again.Identity.SHA256 != request.Identity.SHA256 {
		t.Fatalf("identity must be deterministic: %s != %s", again.Identity.SHA256, request.Identity.SHA256)
	}
}

func TestBuildReleaseDispatchRequestRejectsInvalidInputs(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)

	t.Run("v1", func(t *testing.T) {
		v1 := *ctx
		v1.SourceFormat = releaseconfig.SourceFormatV1
		if _, err := BuildReleaseDispatchRequest(&v1, result); err == nil {
			t.Fatal("expected V1 rejection")
		}
	})

	t.Run("local", func(t *testing.T) {
		local := *ctx
		local.Delivery = "local"
		if _, err := BuildReleaseDispatchRequest(&local, result); err == nil {
			t.Fatal("expected local delivery rejection")
		}
	})

	t.Run("missing workflow", func(t *testing.T) {
		missing := *ctx
		missing.Workflow = ""
		if _, err := BuildReleaseDispatchRequest(&missing, result); err == nil {
			t.Fatal("expected missing workflow rejection")
		}
	})

	t.Run("bad tag spec", func(t *testing.T) {
		bad := *ctx
		bad.Tag = "web/v0.2.1"
		if _, err := BuildReleaseDispatchRequest(&bad, result); err == nil {
			t.Fatal("expected tag spec rejection")
		}
	})

	t.Run("tag different commit", func(t *testing.T) {
		otherRoot := newGitHubActionsDispatchRepository(t)
		otherCtx, otherResult := prepareDispatchRequestContext(t, otherRoot, Patch)
		gitCmd(t, otherRoot, "tag", "-f", otherCtx.Tag, "HEAD~1")
		if _, err := BuildReleaseDispatchRequest(otherCtx, otherResult); err == nil || !strings.Contains(err.Error(), "points to") {
			t.Fatalf("expected tag mismatch, got %v", err)
		}
	})
}

func TestDispatchIdentityChangesOnReleaseFieldsButNotRepositoryPath(t *testing.T) {
	root := newGitHubActionsDispatchRepository(t)
	ctx, result := prepareDispatchRequestContext(t, root, Patch)
	request, err := BuildReleaseDispatchRequest(ctx, result)
	if err != nil {
		t.Fatalf("BuildReleaseDispatchRequest: %v", err)
	}

	tests := []struct { //nolint:govet // Test table order mirrors mutation naming.
		name   string
		mutate func(*ReleaseExecutionContext, *GitReleaseResult)
	}{
		{name: "unit", mutate: func(ctx *ReleaseExecutionContext, result *GitReleaseResult) {
			ctx.Unit.ID = "web"
			result.Unit = "web"
		}},
		{name: "version", mutate: func(ctx *ReleaseExecutionContext, result *GitReleaseResult) {
			ctx.NextVersion = "0.2.2"
			result.Version = "0.2.2"
		}},
		{name: "tag", mutate: func(ctx *ReleaseExecutionContext, result *GitReleaseResult) {
			ctx.Tag = "api/v0.2.2"
			result.Tag = "api/v0.2.2"
		}},
		{name: "commit", mutate: func(_ *ReleaseExecutionContext, result *GitReleaseResult) {
			result.CommitSHA = strings.Repeat("a", 40)
		}},
		{name: "workflow", mutate: func(ctx *ReleaseExecutionContext, _ *GitReleaseResult) {
			ctx.Workflow = ".github/workflows/release-other.yml"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCtx := *ctx
			nextResult := *result
			tt.mutate(&nextCtx, &nextResult)
			identity, identityErr := newReleaseDispatchIdentity(
				request.Identity.RepositoryRemoteName,
				request.Identity.RepositoryRemote,
				nextCtx.Unit.ID,
				nextResult.Version,
				nextResult.Tag,
				nextResult.CommitSHA,
				nextCtx.Workflow,
				nextCtx.Executor,
				nextCtx.Delivery,
			)
			if identityErr != nil {
				t.Fatalf("newReleaseDispatchIdentity: %v", identityErr)
			}
			if identity.SHA256 == request.Identity.SHA256 {
				t.Fatalf("identity did not change for %s", tt.name)
			}
		})
	}

	sameRemote, err := newReleaseDispatchIdentity(
		request.Identity.RepositoryRemoteName,
		request.Identity.RepositoryRemote,
		request.UnitID,
		request.Version,
		request.Tag,
		request.ReleaseCommitSHA,
		request.WorkflowPath,
		request.Executor,
		request.Delivery,
	)
	if err != nil {
		t.Fatalf("newReleaseDispatchIdentity: %v", err)
	}
	if sameRemote.SHA256 != request.Identity.SHA256 {
		t.Fatalf("same remote identity should produce same hash")
	}
}

func newGitHubActionsDispatchRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release-api.yml"), []byte("name: release\n"), 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".goreleaser.yml"), []byte("{}"), 0644); err != nil {
		t.Fatalf("write goreleaser: %v", err)
	}
	config := `{"schemaVersion":2,"units":[{"id":"api","paths":["api/**"],"workingDirectory":".","tagPrefix":"api/v","executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-api.yml"}}]}`
	state := `{"schemaVersion":2,"units":{"api":{"version":"0.2.0"}}}`
	if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte(config), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte(state), 0644); err != nil {
		t.Fatalf("write state: %v", err)
	}
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@example.com")
	gitCmd(t, root, "config", "user.name", "Test User")
	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial")
	gitCmd(t, root, "remote", "add", "origin", "https://github.com/nekoman/repo.git")
	branch := strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
	gitCmd(t, root, "config", "branch."+branch+".remote", "origin")
	gitCmd(t, root, "config", "branch."+branch+".merge", "refs/heads/"+branch)
	return root
}

func prepareDispatchRequestContext(t *testing.T, root string, releaseType Type) (*ReleaseExecutionContext, *GitReleaseResult) {
	t.Helper()
	repository, err := releaseconfig.LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	ctx, err := BuildReleaseExecutionContext(repository, repository.Units[0], releaseType, false)
	if err != nil {
		t.Fatalf("BuildReleaseExecutionContext: %v", err)
	}
	state := NewStateTransaction(root)
	if err := state.CaptureSnapshot(); err != nil {
		t.Fatalf("CaptureSnapshot: %v", err)
	}
	if err := state.WriteUnitVersion(ctx.Unit.ID, ctx.NextVersion); err != nil {
		t.Fatalf("WriteUnitVersion: %v", err)
	}
	gitCmd(t, root, "add", ".neko/release.state.json")
	gitCmd(t, root, "commit", "-m", ReleaseCommitMessage(ctx))
	commitSHA := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	gitCmd(t, root, "tag", ctx.Tag, commitSHA)
	branch := strings.TrimSpace(gitOutput(t, root, "symbolic-ref", "--short", "HEAD"))
	remoteName := strings.TrimSpace(gitOutput(t, root, "config", "--get", "branch."+branch+".remote"))
	remoteURL := strings.TrimSpace(gitOutput(t, root, "remote", "get-url", remoteName))
	return ctx, &GitReleaseResult{
		Unit:                 ctx.Unit.ID,
		Version:              ctx.NextVersion,
		Tag:                  ctx.Tag,
		CommitSHA:            commitSHA,
		RepositoryRemoteName: remoteName,
		RepositoryRemote:     remoteURL,
		CommitCreated:        true,
		TagCreated:           true,
		CommitPushed:         true,
		TagPushed:            true,
		ReachedPhase:         string(ExecutionPhaseCompleted),
	}
}
