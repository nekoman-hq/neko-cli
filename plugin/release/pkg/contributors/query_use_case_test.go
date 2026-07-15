package contributors

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestParseContributorsQueryRequestOwnsRawFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		want  contributorsQueryRequest
	}{
		{name: "missing", flags: nil, want: contributorsQueryRequest{}},
		{name: "wrong type", flags: map[string]any{"unit": true}, want: contributorsQueryRequest{}},
		{name: "unit", flags: map[string]any{"unit": "api"}, want: contributorsQueryRequest{Unit: "api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseContributorsQueryRequest(tt.flags); got != tt.want {
				t.Fatalf("parse = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestContributorsCommandHandlerInvokesOneQueryAndMapsResult(t *testing.T) {
	fixed := time.Date(2026, time.July, 15, 8, 30, 0, 0, time.UTC)
	query := &recordingContributorsQuerier{
		result: contributorsQueryResult{Contributors: []contributorQueryEntry{{Author: "Ada <ada@example.com>", Commits: "3"}}},
	}
	handler := contributorsCommandHandler{query: query, clock: fixedContributorsClock{now: fixed}}

	resp, err := handler.Handle(plugin.Request{Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if query.calls != 1 || query.request != (contributorsQueryRequest{Unit: "api"}) {
		t.Fatalf("query calls/request = %d/%#v", query.calls, query.request)
	}
	if !resp.Metadata.Timestamp.Equal(fixed) || resp.Metadata.Command != "contributors" {
		t.Fatalf("unexpected metadata: %#v", resp.Metadata)
	}
	want := []map[string]any{{"author": "Ada <ada@example.com>", "commits": "3"}}
	if got := resp.Data["items"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestContributorsQueryUseCaseUsesOnlySelectedReadDependency(t *testing.T) {
	repository := contributorsV2Repository()
	repositories := &fakeContributorsRepositoryReader{repository: repository}
	gitReader := &fakeContributorsGitReader{
		pathEntries: []contributorQueryEntry{{Author: "Ada", Commits: "2"}, {Author: "Ben", Commits: "1"}},
	}
	useCase := newContributorsQueryUseCase(repositories, gitReader)

	result, failure := useCase.Query(contributorsQueryRequest{Unit: "api"})
	if failure != nil {
		t.Fatalf("Query failure: %#v", failure)
	}
	if repositories.calls != 1 || repositories.root != "." {
		t.Fatalf("repository calls/root = %d/%q", repositories.calls, repositories.root)
	}
	if gitReader.pathCalls != 1 || gitReader.repositoryCalls != 0 || !reflect.DeepEqual(gitReader.paths, []string{"api/**"}) {
		t.Fatalf("unexpected Git reads: %#v", gitReader)
	}
	if !reflect.DeepEqual(result.Contributors, gitReader.pathEntries) {
		t.Fatalf("contributors = %#v, want %#v", result.Contributors, gitReader.pathEntries)
	}
	gitReader.pathEntries[0].Author = "mutated fake"
	if result.Contributors[0].Author != "Ada" {
		t.Fatalf("result aliases dependency slice: %#v", result.Contributors)
	}
}

func TestContributorsQueryUseCaseUsesLegacyRepositoryQuery(t *testing.T) {
	repository := &config.ReleaseRepository{
		SourceFormat: config.SourceFormatV1,
		Units:        []config.ReleaseUnit{{ID: "default", Paths: []string{"**"}}},
	}
	gitReader := &fakeContributorsGitReader{
		repositoryEntries: []contributorQueryEntry{{Author: "Ada", Commits: "4"}},
	}
	useCase := newContributorsQueryUseCase(&fakeContributorsRepositoryReader{repository: repository}, gitReader)

	result, failure := useCase.Query(contributorsQueryRequest{})
	if failure != nil {
		t.Fatalf("Query failure: %#v", failure)
	}
	if gitReader.repositoryCalls != 1 || gitReader.pathCalls != 0 {
		t.Fatalf("unexpected Git reads: %#v", gitReader)
	}
	if !reflect.DeepEqual(result.Contributors, gitReader.repositoryEntries) {
		t.Fatalf("contributors = %#v, want %#v", result.Contributors, gitReader.repositoryEntries)
	}
}

func TestContributorsQueryUseCaseStopsAtFocusedFailures(t *testing.T) {
	t.Run("repository", func(t *testing.T) {
		gitReader := &fakeContributorsGitReader{}
		useCase := newContributorsQueryUseCase(&fakeContributorsRepositoryReader{err: errors.New("load failed")}, gitReader)
		_, failure := useCase.Query(contributorsQueryRequest{})
		assertContributorsFailure(t, failure, "CONFIG_INVALID")
		if gitReader.totalCalls() != 0 {
			t.Fatalf("Git called after repository failure: %#v", gitReader)
		}
	})

	t.Run("unit resolution", func(t *testing.T) {
		gitReader := &fakeContributorsGitReader{}
		useCase := newContributorsQueryUseCase(&fakeContributorsRepositoryReader{repository: contributorsV2Repository()}, gitReader)
		_, failure := useCase.Query(contributorsQueryRequest{})
		assertContributorsFailure(t, failure, "UNIT_RESOLUTION_FAILED")
		if gitReader.totalCalls() != 0 {
			t.Fatalf("Git called after unit failure: %#v", gitReader)
		}
	})

	t.Run("git", func(t *testing.T) {
		gitReader := &fakeContributorsGitReader{pathErr: errors.New("shortlog failed")}
		useCase := newContributorsQueryUseCase(&fakeContributorsRepositoryReader{repository: contributorsV2Repository()}, gitReader)
		_, failure := useCase.Query(contributorsQueryRequest{Unit: "api"})
		assertContributorsFailure(t, failure, "GIT_CONTRIBUTORS_FAILED")
		if gitReader.pathCalls != 1 || gitReader.repositoryCalls != 0 {
			t.Fatalf("unexpected Git calls: %#v", gitReader)
		}
	})
}

//nolint:govet // Test fake field order groups returned values before captured calls.
type recordingContributorsQuerier struct {
	result  contributorsQueryResult
	failure *contributorsQueryFailure
	request contributorsQueryRequest
	calls   int
}

func (query *recordingContributorsQuerier) Query(request contributorsQueryRequest) (contributorsQueryResult, *contributorsQueryFailure) {
	query.calls++
	query.request = request
	return query.result, query.failure
}

type fixedContributorsClock struct{ now time.Time }

func (clock fixedContributorsClock) Now() time.Time { return clock.now }

type fakeContributorsRepositoryReader struct {
	repository *config.ReleaseRepository
	err        error
	root       string
	calls      int
}

func (reader *fakeContributorsRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	reader.calls++
	reader.root = root
	return reader.repository, reader.err
}

type fakeContributorsGitReader struct {
	repositoryEntries []contributorQueryEntry
	pathEntries       []contributorQueryEntry
	repositoryErr     error
	pathErr           error
	paths             []string
	repositoryCalls   int
	pathCalls         int
}

func (reader *fakeContributorsGitReader) ForRepository() ([]contributorQueryEntry, error) {
	reader.repositoryCalls++
	return reader.repositoryEntries, reader.repositoryErr
}

func (reader *fakeContributorsGitReader) ForPaths(paths []string) ([]contributorQueryEntry, error) {
	reader.pathCalls++
	reader.paths = append([]string(nil), paths...)
	return reader.pathEntries, reader.pathErr
}

func (reader *fakeContributorsGitReader) totalCalls() int {
	return reader.repositoryCalls + reader.pathCalls
}

func contributorsV2Repository() *config.ReleaseRepository {
	return &config.ReleaseRepository{
		SourceFormat: config.SourceFormatV2,
		Units: []config.ReleaseUnit{
			{ID: "api", Paths: []string{"api/**"}},
			{ID: "web", Paths: []string{"web/**"}},
		},
	}
}

func assertContributorsFailure(t *testing.T, failure *contributorsQueryFailure, code string) {
	t.Helper()
	if failure == nil || failure.Code != code {
		t.Fatalf("failure = %#v, want %s", failure, code)
	}
}
