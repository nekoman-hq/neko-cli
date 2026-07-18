package release

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestReleaseContextValidationAcceptsLightweightAndAnnotatedTagsAndDetachedHead(t *testing.T) {
	for _, annotated := range []bool{false, true} {
		t.Run(map[bool]string{false: "lightweight", true: "annotated"}[annotated], func(t *testing.T) {
			root, request := newRealReleaseContextRepository(t, annotated)
			gitCmd(t, root, "checkout", "--detach", request.ReleaseSHA)
			statusBefore := gitOutput(t, root, "status", "--porcelain", "--untracked-files=all")
			refsBefore := gitOutput(t, root, "show-ref")
			configBefore := readReleaseContextTestFile(t, releaseconfig.V2ConfigPath(root))
			stateBefore := readReleaseContextTestFile(t, releaseconfig.V2StatePath(root))

			result, failure := newReleaseContextValidationUseCase().Validate(context.Background(), request)
			if failure != nil || result == nil || !result.HeadMatches || !result.TagTargetMatches {
				t.Fatalf("result=%#v failure=%#v", result, failure)
			}
			if got := gitOutput(t, root, "status", "--porcelain", "--untracked-files=all"); got != statusBefore {
				t.Fatalf("worktree or index changed: before=%q after=%q", statusBefore, got)
			}
			if got := gitOutput(t, root, "show-ref"); got != refsBefore {
				t.Fatalf("refs changed: before=%q after=%q", refsBefore, got)
			}
			if got := readReleaseContextTestFile(t, releaseconfig.V2ConfigPath(root)); got != configBefore {
				t.Fatal("V2 config changed during validation")
			}
			if got := readReleaseContextTestFile(t, releaseconfig.V2StatePath(root)); got != stateBefore {
				t.Fatal("V2 state changed during validation")
			}
		})
	}
}

func TestReleaseContextValidationAcceptsRepositoryPathWithSpaces(t *testing.T) {
	root := filepath.Join(t.TempDir(), "repository with spaces")
	request := newRealReleaseContextRepositoryAt(t, root, false)

	result, failure := newReleaseContextValidationUseCase().Validate(context.Background(), request)
	if failure != nil || result == nil || result.ReleaseSHA != request.ReleaseSHA {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
}

func TestReleaseContextValidationRejectsRealGitMismatchesAndNonCommitObjects(t *testing.T) {
	t.Run("head mismatch", func(t *testing.T) {
		root, request := newRealReleaseContextRepository(t, false)
		if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("other\n"), 0o644); err != nil {
			t.Fatalf("write other: %v", err)
		}
		gitCmd(t, root, "add", "other.txt")
		gitCmd(t, root, "commit", "-m", "other")
		assertReleaseContextFailureCode(t, newReleaseContextValidationUseCase(), request, "HEAD_MISMATCH")
	})

	t.Run("missing tag", func(t *testing.T) {
		root, request := newRealReleaseContextRepository(t, false)
		gitCmd(t, root, "tag", "-d", request.Tag)
		assertReleaseContextFailureCode(t, newReleaseContextValidationUseCase(), request, "RELEASE_TAG_MISSING")
	})

	t.Run("tag target mismatch", func(t *testing.T) {
		root, request := newRealReleaseContextRepository(t, false)
		if err := os.WriteFile(filepath.Join(root, "other.txt"), []byte("other\n"), 0o644); err != nil {
			t.Fatalf("write other: %v", err)
		}
		gitCmd(t, root, "add", "other.txt")
		gitCmd(t, root, "commit", "-m", "other")
		other := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
		gitCmd(t, root, "tag", "-f", request.Tag, other)
		gitCmd(t, root, "checkout", "--detach", request.ReleaseSHA)
		assertReleaseContextFailureCode(t, newReleaseContextValidationUseCase(), request, "TAG_TARGET_MISMATCH")
	})

	t.Run("release sha is blob", func(t *testing.T) {
		root, request := newRealReleaseContextRepository(t, false)
		blob := strings.TrimSpace(gitOutput(t, root, "hash-object", "README.md"))
		request.ReleaseSHA = blob
		assertReleaseContextFailureCode(t, newReleaseContextValidationUseCase(), request, "RELEASE_SHA_NOT_COMMIT")
	})
}

func TestReleaseContextSourceRequiresCompleteUnblockedV2Pair(t *testing.T) {
	t.Run("missing and malformed V2 files", func(t *testing.T) {
		tests := []struct {
			name   string
			change func(*testing.T, string)
			code   string
		}{
			{name: "missing config", change: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Remove(releaseconfig.V2ConfigPath(root)); err != nil {
					t.Fatalf("remove config: %v", err)
				}
			}, code: "V2_CONTEXT_SOURCE_MISSING"},
			{name: "missing state", change: func(t *testing.T, root string) {
				t.Helper()
				if err := os.Remove(releaseconfig.V2StatePath(root)); err != nil {
					t.Fatalf("remove state: %v", err)
				}
			}, code: "V2_CONTEXT_SOURCE_MISSING"},
			{name: "malformed config", change: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(releaseconfig.V2ConfigPath(root), []byte("{malformed"), 0o644); err != nil {
					t.Fatalf("write malformed config: %v", err)
				}
			}, code: "V2_CONFIGURATION_INVALID"},
			{name: "malformed state", change: func(t *testing.T, root string) {
				t.Helper()
				if err := os.WriteFile(releaseconfig.V2StatePath(root), []byte("{malformed"), 0o644); err != nil {
					t.Fatalf("write malformed state: %v", err)
				}
			}, code: "V2_STATE_INVALID"},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				root, _ := newRealReleaseContextRepository(t, false)
				test.change(t, root)
				_, failure := (filesystemReleaseContextSourceReader{}).ReadV2(root)
				if failure == nil || failure.Code != test.code {
					t.Fatalf("failure = %#v, want %s", failure, test.code)
				}
			})
		}
	})

	t.Run("V1 only", func(t *testing.T) {
		root := newContextCharacterizationRepository(t)
		secret := "V1_CONTENT_MUST_NOT_BE_READ"
		if err := os.WriteFile(filepath.Join(root, releaseconfig.V1FileName), []byte(secret), 0o000); err != nil { //nolint:staticcheck
			t.Fatalf("write V1 marker: %v", err)
		}
		_, failure := (filesystemReleaseContextSourceReader{}).ReadV2(root)
		if failure == nil || failure.Code != "UNSUPPORTED_RELEASE_SOURCE" || strings.Contains(failure.responseMessage(), secret) {
			t.Fatalf("failure = %#v", failure)
		}
	})

	t.Run("V1 and V2 conflict", func(t *testing.T) {
		root, _ := newRealReleaseContextRepository(t, false)
		if err := os.WriteFile(filepath.Join(root, releaseconfig.V1FileName), []byte("{}\n"), 0o644); err != nil { //nolint:staticcheck
			t.Fatalf("write V1 marker: %v", err)
		}
		_, failure := (filesystemReleaseContextSourceReader{}).ReadV2(root)
		if failure == nil || failure.Code != "V2_CONTEXT_SOURCE_CONFLICT" {
			t.Fatalf("failure = %#v", failure)
		}
	})

	t.Run("config state mismatch", func(t *testing.T) {
		root, _ := newRealReleaseContextRepository(t, false)
		state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"web": {Version: "1.2.3"}}}
		writeReleaseContextJSON(t, releaseconfig.V2StatePath(root), state)
		_, failure := (filesystemReleaseContextSourceReader{}).ReadV2(root)
		if failure == nil || failure.Code != "V2_CONFIG_STATE_MISMATCH" {
			t.Fatalf("failure = %#v", failure)
		}
	})

	t.Run("empty tag prefix", func(t *testing.T) {
		root, _ := newRealReleaseContextRepository(t, false)
		cfg := releaseconfig.V2ReleaseConfig{
			SchemaVersion: 2,
			Units: []releaseconfig.V2Unit{{
				ID:               "api",
				DisplayName:      "API",
				Paths:            []string{"**"},
				WorkingDirectory: ".",
				TagPrefix:        "",
				Executor: releaseconfig.V2Executor{
					Type:     releaseconfig.ExecutorGoReleaser,
					Delivery: releaseconfig.DeliveryGitHubActions,
					Workflow: ".github/workflows/release.yml",
				},
			}},
		}
		writeReleaseContextJSON(t, releaseconfig.V2ConfigPath(root), cfg)
		_, failure := (filesystemReleaseContextSourceReader{}).ReadV2(root)
		if failure == nil || failure.Code != "V2_CONTEXT_SOURCE_INVALID" {
			t.Fatalf("failure = %#v", failure)
		}
	})

	t.Run("pair recovery blocked", func(t *testing.T) {
		root, _ := newRealReleaseContextRepository(t, false)
		if err := os.WriteFile(releaseconfig.V2PairRecoveryPath(root), []byte("{malformed"), 0o644); err != nil {
			t.Fatalf("write pair evidence: %v", err)
		}
		_, failure := (filesystemReleaseContextSourceReader{}).ReadV2(root)
		if failure == nil || failure.Code != "V2_CONTEXT_RECOVERY_BLOCKED" {
			t.Fatalf("failure = %#v", failure)
		}
	})
}

func TestReleaseContextValidationKeepsExplicitRepositoryRootsIsolated(t *testing.T) {
	firstRoot, firstRequest := newRealReleaseContextRepository(t, false)
	secondRoot, secondRequest := newRealReleaseContextRepository(t, true)
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	useCase := newReleaseContextValidationUseCase()
	first, firstFailure := useCase.Validate(context.Background(), firstRequest)
	second, secondFailure := useCase.Validate(context.Background(), secondRequest)
	if firstFailure != nil || secondFailure != nil || first.ReleaseSHA != firstRequest.ReleaseSHA || second.ReleaseSHA != secondRequest.ReleaseSHA {
		t.Fatalf("first=%#v/%#v second=%#v/%#v", first, firstFailure, second, secondFailure)
	}
	if got, err := os.Getwd(); err != nil || got != workingDirectory {
		t.Fatalf("cwd = %q err=%v, want %q", got, err, workingDirectory)
	}
	if firstRoot == secondRoot {
		t.Fatal("test repositories unexpectedly share a root")
	}
}

func newRealReleaseContextRepository(t *testing.T, annotated bool) (string, ReleaseContextValidationRequest) {
	t.Helper()
	root := newContextCharacterizationRepository(t)
	return root, populateRealReleaseContextRepository(t, root, annotated)
}

func newRealReleaseContextRepositoryAt(t *testing.T, root string, annotated bool) ReleaseContextValidationRequest {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir repository: %v", err)
	}
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "release-context@example.invalid")
	gitCmd(t, root, "config", "user.name", "Release Context")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("release context\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitCmd(t, root, "add", "README.md")
	gitCmd(t, root, "commit", "-m", "initial")
	return populateRealReleaseContextRepository(t, root, annotated)
}

func populateRealReleaseContextRepository(t *testing.T, root string, annotated bool) ReleaseContextValidationRequest {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".neko"), 0o755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".github", "workflows", "release.yml"), []byte("name: release\n"), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	cfg := releaseconfig.V2ReleaseConfig{
		SchemaVersion: 2,
		Units: []releaseconfig.V2Unit{{
			ID:               "api",
			DisplayName:      "API μservice",
			Paths:            []string{"**"},
			WorkingDirectory: ".",
			TagPrefix:        "api/v",
			Executor: releaseconfig.V2Executor{
				Type:     releaseconfig.ExecutorGoReleaser,
				Delivery: releaseconfig.DeliveryGitHubActions,
				Workflow: ".github/workflows/release.yml",
			},
		}},
	}
	state := releaseconfig.V2ReleaseState{SchemaVersion: 2, Units: map[string]releaseconfig.V2UnitState{"api": {Version: "1.2.3"}}}
	writeReleaseContextJSON(t, releaseconfig.V2ConfigPath(root), cfg)
	writeReleaseContextJSON(t, releaseconfig.V2StatePath(root), state)
	gitCmd(t, root, "add", ".neko/release.config.json", ".neko/release.state.json", ".github/workflows/release.yml")
	gitCmd(t, root, "commit", "-m", "release context")
	sha := strings.TrimSpace(gitOutput(t, root, "rev-parse", "HEAD"))
	if annotated {
		gitCmd(t, root, "tag", "-a", "api/v1.2.3", "-m", "release", sha)
	} else {
		gitCmd(t, root, "tag", "api/v1.2.3", sha)
	}
	return ReleaseContextValidationRequest{
		RepositoryRoot: root,
		UnitID:         "api",
		Version:        "1.2.3",
		Tag:            "api/v1.2.3",
		ReleaseSHA:     sha,
	}
}

func writeReleaseContextJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readReleaseContextTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func assertReleaseContextFailureCode(t *testing.T, useCase releaseContextValidationUseCase, request ReleaseContextValidationRequest, want string) {
	t.Helper()
	result, failure := useCase.Validate(context.Background(), request)
	if result != nil || failure == nil || failure.Code != want {
		t.Fatalf("result=%#v failure=%#v, want %s", result, failure, want)
	}
}
