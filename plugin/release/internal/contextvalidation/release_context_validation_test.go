package contextvalidation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestReleaseContextValidationUseCaseReturnsCanonicalContext(t *testing.T) {
	sources := &recordingReleaseContextSource{repository: contextValidationRepository()}
	git := &recordingReleaseContextGit{
		objectFormat: GitObjectFormatSHA1,
		objectType:   "commit",
		head:         strings.Repeat("a", 40),
		tagExists:    true,
		tagCommit:    strings.Repeat("a", 40),
	}
	useCase := releaseContextValidationUseCase{sources: sources, git: git}

	result, failure := useCase.Validate(context.Background(), validReleaseContextRequest())
	if failure != nil {
		t.Fatalf("Validate failure: %#v", failure)
	}
	want := &ValidatedReleaseContext{
		UnitID:           "api",
		DisplayName:      "API",
		Version:          "1.2.3",
		TagPrefix:        "api/v",
		Tag:              "api/v1.2.3",
		ReleaseSHA:       strings.Repeat("a", 40),
		WorkingDirectory: "services/api",
		Executor:         "goreleaser",
		Delivery:         "github-actions",
		Workflow:         ".github/workflows/release.yml",
		GitObjectFormat:  GitObjectFormatSHA1,
		HeadMatches:      true,
		TagTargetMatches: true,
	}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}
	if sources.calls != 1 {
		t.Fatalf("source reads = %d, want 1", sources.calls)
	}
	wantTrace := []string{"object-format", "object-type", "head", "tag-exists", "tag-commit"}
	if !reflect.DeepEqual(git.trace, wantTrace) {
		t.Fatalf("Git trace = %v, want %v", git.trace, wantTrace)
	}

	again, secondFailure := useCase.Validate(context.Background(), validReleaseContextRequest())
	if secondFailure != nil || !reflect.DeepEqual(again, result) {
		t.Fatalf("second result = %#v failure=%#v, want deterministic %#v", again, secondFailure, result)
	}
}

func TestReleaseContextValidationUseCaseRejectsContractMismatches(t *testing.T) {
	validSHA := strings.Repeat("a", 40)
	tests := []struct {
		name       string
		mutate     func(*ReleaseContextValidationRequest, *releaseconfig.ReleaseRepository, *recordingReleaseContextGit)
		wantCode   string
		wantGitOps int
	}{
		{name: "missing unit", mutate: func(request *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, _ *recordingReleaseContextGit) {
			request.UnitID = "missing"
		}, wantCode: "RELEASE_UNIT_NOT_FOUND"},
		{name: "version mismatch", mutate: func(request *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, _ *recordingReleaseContextGit) {
			request.Version = "1.2.4"
		}, wantCode: "RELEASE_VERSION_MISMATCH"},
		{name: "tag mismatch", mutate: func(request *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, _ *recordingReleaseContextGit) {
			request.Tag = "web/v1.2.3"
		}, wantCode: "RELEASE_TAG_MISMATCH"},
		{name: "release sha object missing", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.objectTypeErr = errors.New("missing")
		}, wantCode: "RELEASE_SHA_NOT_COMMIT", wantGitOps: 2},
		{name: "release sha is blob", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.objectType = "blob"
		}, wantCode: "RELEASE_SHA_NOT_COMMIT", wantGitOps: 2},
		{name: "head unavailable", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.headErr = errors.New("unborn")
		}, wantCode: "HEAD_UNAVAILABLE", wantGitOps: 3},
		{name: "head mismatch", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.head = strings.Repeat("b", 40)
		}, wantCode: "HEAD_MISMATCH", wantGitOps: 3},
		{name: "tag history unavailable", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.tagExistsErr = errors.New("git failure")
		}, wantCode: "TAG_HISTORY_UNAVAILABLE", wantGitOps: 4},
		{name: "tag missing", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.tagExists = false
		}, wantCode: "RELEASE_TAG_MISSING", wantGitOps: 4},
		{name: "tag does not resolve to commit", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.tagCommitErr = errors.New("not a commit")
		}, wantCode: "TAG_TARGET_INVALID", wantGitOps: 5},
		{name: "tag target mismatch", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.tagCommit = strings.Repeat("b", 40)
		}, wantCode: "TAG_TARGET_MISMATCH", wantGitOps: 5},
		{name: "unsupported object format", mutate: func(_ *ReleaseContextValidationRequest, _ *releaseconfig.ReleaseRepository, git *recordingReleaseContextGit) {
			git.objectFormat = "unknown"
		}, wantCode: "GIT_OBJECT_FORMAT_UNSUPPORTED", wantGitOps: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validReleaseContextRequest()
			repository := contextValidationRepository()
			git := &recordingReleaseContextGit{
				objectFormat: GitObjectFormatSHA1,
				objectType:   "commit",
				head:         validSHA,
				tagExists:    true,
				tagCommit:    validSHA,
			}
			test.mutate(&request, repository, git)
			useCase := releaseContextValidationUseCase{
				sources: &recordingReleaseContextSource{repository: repository},
				git:     git,
			}

			result, failure := useCase.Validate(context.Background(), request)
			if result != nil || failure == nil || failure.Code != test.wantCode {
				t.Fatalf("result=%#v failure=%#v, want code %q", result, failure, test.wantCode)
			}
			if len(git.trace) != test.wantGitOps {
				t.Fatalf("Git operations = %v, want %d", git.trace, test.wantGitOps)
			}
		})
	}
}

func TestReleaseContextValidationUseCaseRejectsUnsafeInputBeforeDependencies(t *testing.T) {
	longUnit := "a" + strings.Repeat("b", 16_384)
	tests := []struct {
		name   string
		mutate func(*ReleaseContextValidationRequest)
		code   string
	}{
		{name: "missing unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = "" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "whitespace version", mutate: func(request *ReleaseContextValidationRequest) { request.Version = " 1.2.3" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "newline tag", mutate: func(request *ReleaseContextValidationRequest) { request.Tag = "api/v1.2.3\nother=value" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "carriage return tag", mutate: func(request *ReleaseContextValidationRequest) { request.Tag = "api/v1.2.3\rother=value" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "shell unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = "api;touch-owned" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "quoted unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = `api"quoted` }, code: "INVALID_CONTEXT_INPUT"},
		{name: "command substitution unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = "api$(touch-owned)" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "unicode unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = "äpı" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "traversal unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = "../api" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "option unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = "--api" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "long unknown unit", mutate: func(request *ReleaseContextValidationRequest) { request.UnitID = longUnit }, code: "RELEASE_UNIT_NOT_FOUND"},
		{name: "malformed version", mutate: func(request *ReleaseContextValidationRequest) { request.Version = "1.2" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "prefixed version", mutate: func(request *ReleaseContextValidationRequest) { request.Version = "v1.2.3" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "noncanonical version", mutate: func(request *ReleaseContextValidationRequest) { request.Version = "01.2.3" }, code: "INVALID_CONTEXT_INPUT"},
		{name: "abbreviated sha", mutate: func(request *ReleaseContextValidationRequest) { request.ReleaseSHA = "abcdef0" }, code: "INVALID_RELEASE_SHA"},
		{name: "uppercase sha", mutate: func(request *ReleaseContextValidationRequest) { request.ReleaseSHA = strings.Repeat("A", 40) }, code: "INVALID_RELEASE_SHA"},
		{name: "option sha", mutate: func(request *ReleaseContextValidationRequest) { request.ReleaseSHA = "--help" }, code: "INVALID_RELEASE_SHA"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validReleaseContextRequest()
			test.mutate(&request)
			sources := &recordingReleaseContextSource{repository: contextValidationRepository()}
			git := &recordingReleaseContextGit{}
			useCase := releaseContextValidationUseCase{sources: sources, git: git}

			result, failure := useCase.Validate(context.Background(), request)
			if result != nil || failure == nil || failure.Code != test.code {
				t.Fatalf("result=%#v failure=%#v, want code %q", result, failure, test.code)
			}
			if test.code != "RELEASE_UNIT_NOT_FOUND" && sources.calls != 0 {
				t.Fatalf("source reads = %d, want 0", sources.calls)
			}
			if len(git.trace) != 0 {
				t.Fatalf("unsafe input reached Git: %v", git.trace)
			}
		})
	}
}

func TestReleaseContextValidationUseCaseSupportsRepositoryObjectIDFormats(t *testing.T) {
	for _, test := range []struct {
		name   string
		format GitObjectFormat
		sha    string
	}{
		{name: "sha1", format: GitObjectFormatSHA1, sha: strings.Repeat("a", 40)},
		{name: "sha256", format: GitObjectFormatSHA256, sha: strings.Repeat("b", 64)},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := validReleaseContextRequest()
			request.ReleaseSHA = test.sha
			git := &recordingReleaseContextGit{
				objectFormat: test.format,
				objectType:   "commit",
				head:         test.sha,
				tagExists:    true,
				tagCommit:    test.sha,
			}
			useCase := releaseContextValidationUseCase{
				sources: &recordingReleaseContextSource{repository: contextValidationRepository()},
				git:     git,
			}
			result, failure := useCase.Validate(context.Background(), request)
			if failure != nil || result == nil || result.GitObjectFormat != test.format {
				t.Fatalf("result=%#v failure=%#v", result, failure)
			}
		})
	}
}

func TestReleaseContextValidationComparesCanonicalAuthoritativeVersion(t *testing.T) {
	repository := contextValidationRepository()
	repository.Units[0].Version = "v1.2.3"
	sha := strings.Repeat("a", 40)
	git := &recordingReleaseContextGit{
		objectFormat: GitObjectFormatSHA1,
		objectType:   "commit",
		head:         sha,
		tagExists:    true,
		tagCommit:    sha,
	}
	useCase := releaseContextValidationUseCase{
		sources: &recordingReleaseContextSource{repository: repository},
		git:     git,
	}

	result, failure := useCase.Validate(context.Background(), validReleaseContextRequest())
	if failure != nil || result == nil || result.Version != "1.2.3" {
		t.Fatalf("result=%#v failure=%#v", result, failure)
	}
}

type recordingReleaseContextSource struct {
	repository *releaseconfig.ReleaseRepository
	failure    *commandFailure
	calls      int
}

func (source *recordingReleaseContextSource) ReadV2(string) (*releaseconfig.ReleaseRepository, *commandFailure) {
	source.calls++
	return source.repository, source.failure
}

type recordingReleaseContextGit struct {
	objectFormat    GitObjectFormat
	objectFormatErr error
	objectType      string
	objectTypeErr   error
	head            string
	headErr         error
	tagExistsErr    error
	tagCommit       string
	tagCommitErr    error
	trace           []string
	tagExists       bool
}

func (git *recordingReleaseContextGit) ObjectFormat(string) (GitObjectFormat, error) {
	git.trace = append(git.trace, "object-format")
	return git.objectFormat, git.objectFormatErr
}

func (git *recordingReleaseContextGit) ObjectType(string, string) (string, error) {
	git.trace = append(git.trace, "object-type")
	return git.objectType, git.objectTypeErr
}

func (git *recordingReleaseContextGit) HeadCommit(string) (string, error) {
	git.trace = append(git.trace, "head")
	return git.head, git.headErr
}

func (git *recordingReleaseContextGit) TagExists(string, string) (bool, error) {
	git.trace = append(git.trace, "tag-exists")
	return git.tagExists, git.tagExistsErr
}

func (git *recordingReleaseContextGit) TagCommit(string, string) (string, error) {
	git.trace = append(git.trace, "tag-commit")
	return git.tagCommit, git.tagCommitErr
}

func contextValidationRepository() *releaseconfig.ReleaseRepository {
	return &releaseconfig.ReleaseRepository{
		RepositoryRoot: "/repo",
		SchemaVersion:  2,
		SourceFormat:   releaseconfig.SourceFormatV2,
		Units: []releaseconfig.ReleaseUnit{
			{
				ID:               "api",
				DisplayName:      "API",
				Version:          "1.2.3",
				TagPrefix:        "api/v",
				WorkingDirectory: "services/api",
				ExecutorType:     "goreleaser",
				Delivery:         "github-actions",
				Workflow:         ".github/workflows/release.yml",
			},
			{ID: "web", Version: "2.0.0", TagPrefix: "web/v"},
		},
	}
}

func validReleaseContextRequest() ReleaseContextValidationRequest {
	return ReleaseContextValidationRequest{
		RepositoryRoot: "/repo",
		UnitID:         "api",
		Version:        "1.2.3",
		Tag:            "api/v1.2.3",
		ReleaseSHA:     strings.Repeat("a", 40),
	}
}
