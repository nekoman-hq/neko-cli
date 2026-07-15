package history

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestParseHistoryQueryRequestOwnsRawFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		want  historyQueryRequest
	}{
		{name: "missing", flags: nil, want: historyQueryRequest{}},
		{name: "wrong type", flags: map[string]any{"unit": false}, want: historyQueryRequest{}},
		{name: "unit", flags: map[string]any{"unit": "api"}, want: historyQueryRequest{Unit: "api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseHistoryQueryRequest(tt.flags); got != tt.want {
				t.Fatalf("parse = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestHistoryCommandHandlerInvokesOneQueryAndMapsFailure(t *testing.T) {
	fixed := time.Date(2026, time.July, 15, 9, 0, 0, 0, time.UTC)
	query := &recordingHistoryQuerier{failure: &historyQueryFailure{Code: "GIT_HISTORY_FAILED", Message: "git failed"}}
	handler := historyCommandHandler{query: query, clock: fixedHistoryClock{now: fixed}}

	resp, err := handler.Handle(plugin.Request{Flags: map[string]any{"unit": "api"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if query.calls != 1 || query.request != (historyQueryRequest{Unit: "api"}) {
		t.Fatalf("query calls/request = %d/%#v", query.calls, query.request)
	}
	if resp.Status != "error" || resp.Error == nil || resp.Error.Code != "GIT_HISTORY_FAILED" || resp.Error.Message != "git failed" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if !resp.Metadata.Timestamp.Equal(fixed) || resp.Metadata.Command != "history" {
		t.Fatalf("unexpected metadata: %#v", resp.Metadata)
	}
}

func TestHistoryQueryUseCaseReturnsOrderedV2History(t *testing.T) {
	repositories := &fakeHistoryRepositoryReader{repository: historyV2Repository()}
	gitReader := &fakeHistoryGitReader{
		unitTags: []historyUnitTag{
			{Tag: "api/v0.1.0"},
			{Tag: "api/v0.2.0"},
		},
		unitCounts: []int{3, 2},
	}
	useCase := newHistoryQueryUseCase(repositories, gitReader)

	result, failure := useCase.Query(historyQueryRequest{Unit: "api"})
	if failure != nil {
		t.Fatalf("Query failure: %#v", failure)
	}
	want := []historyQueryEntry{
		{Unit: "api", Version: "api/v0.1.0", From: "", Commits: 3},
		{Unit: "api", Version: "api/v0.2.0", From: "api/v0.1.0", Commits: 2},
	}
	if result.SourceFormat != config.SourceFormatV2 || !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("result = %#v, want entries %#v", result, want)
	}
	if repositories.calls != 1 || repositories.root != "." || gitReader.unitTagsCalls != 1 {
		t.Fatalf("unexpected dependency calls: repo=%#v git=%#v", repositories, gitReader)
	}
	if gitReader.spec.Prefix != "api/v" || !reflect.DeepEqual(gitReader.countCalls, []historyCountCall{
		{from: "", to: "api/v0.1.0", paths: []string{"api/**"}},
		{from: "api/v0.1.0", to: "api/v0.2.0", paths: []string{"api/**"}},
	}) {
		t.Fatalf("unexpected Git queries: %#v", gitReader)
	}
}

func TestHistoryQueryUseCasePreservesLegacyNonErroringQueries(t *testing.T) {
	repository := &config.ReleaseRepository{
		SourceFormat: config.SourceFormatV1,
		Units:        []config.ReleaseUnit{{ID: "default"}},
	}
	gitReader := &fakeHistoryGitReader{legacyTags: []string{"v1.0.0", "v1.1.0"}, legacyCounts: []int{5, 2}}
	useCase := newHistoryQueryUseCase(&fakeHistoryRepositoryReader{repository: repository}, gitReader)

	result, failure := useCase.Query(historyQueryRequest{})
	if failure != nil {
		t.Fatalf("Query failure: %#v", failure)
	}
	want := []historyQueryEntry{
		{Version: "v1.0.0", From: "", Commits: 5},
		{Version: "v1.1.0", From: "v1.0.0", Commits: 2},
	}
	if result.SourceFormat != config.SourceFormatV1 || !reflect.DeepEqual(result.Entries, want) {
		t.Fatalf("result = %#v, want %#v", result, want)
	}
	if gitReader.legacyTagsCalls != 1 || len(gitReader.legacyCountCalls) != 2 || gitReader.unitTagsCalls != 0 {
		t.Fatalf("unexpected Git calls: %#v", gitReader)
	}
}

func TestHistoryQueryUseCaseStopsAtFocusedFailures(t *testing.T) {
	t.Run("repository", func(t *testing.T) {
		gitReader := &fakeHistoryGitReader{}
		useCase := newHistoryQueryUseCase(&fakeHistoryRepositoryReader{err: errors.New("load failed")}, gitReader)
		_, failure := useCase.Query(historyQueryRequest{})
		assertHistoryQueryFailure(t, failure, "CONFIG_INVALID")
		if gitReader.totalCalls() != 0 {
			t.Fatalf("Git called after repository failure: %#v", gitReader)
		}
	})

	t.Run("unit resolution", func(t *testing.T) {
		gitReader := &fakeHistoryGitReader{}
		useCase := newHistoryQueryUseCase(&fakeHistoryRepositoryReader{repository: historyV2Repository()}, gitReader)
		_, failure := useCase.Query(historyQueryRequest{})
		assertHistoryQueryFailure(t, failure, "UNIT_RESOLUTION_FAILED")
		if gitReader.totalCalls() != 0 {
			t.Fatalf("Git called after unit failure: %#v", gitReader)
		}
	})

	t.Run("tag query", func(t *testing.T) {
		gitReader := &fakeHistoryGitReader{unitTagsErr: errors.New("log failed")}
		useCase := newHistoryQueryUseCase(&fakeHistoryRepositoryReader{repository: historyV2Repository()}, gitReader)
		_, failure := useCase.Query(historyQueryRequest{Unit: "api"})
		assertHistoryQueryFailure(t, failure, "GIT_HISTORY_FAILED")
		if gitReader.unitTagsCalls != 1 || len(gitReader.countCalls) != 0 {
			t.Fatalf("commit counts called after tag failure: %#v", gitReader)
		}
	})

	t.Run("commit count", func(t *testing.T) {
		gitReader := &fakeHistoryGitReader{
			unitTags:     []historyUnitTag{{Tag: "api/v0.1.0"}, {Tag: "api/v0.2.0"}},
			unitCounts:   []int{1},
			unitCountErr: map[int]error{1: errors.New("count failed")},
		}
		useCase := newHistoryQueryUseCase(&fakeHistoryRepositoryReader{repository: historyV2Repository()}, gitReader)
		_, failure := useCase.Query(historyQueryRequest{Unit: "api"})
		assertHistoryQueryFailure(t, failure, "GIT_HISTORY_FAILED")
		if len(gitReader.countCalls) != 2 {
			t.Fatalf("count calls = %d, want stop at second", len(gitReader.countCalls))
		}
	})
}

//nolint:govet // Test fake field order groups returned values before captured calls.
type recordingHistoryQuerier struct {
	result  historyQueryResult
	failure *historyQueryFailure
	request historyQueryRequest
	calls   int
}

func (query *recordingHistoryQuerier) Query(request historyQueryRequest) (historyQueryResult, *historyQueryFailure) {
	query.calls++
	query.request = request
	return query.result, query.failure
}

type fixedHistoryClock struct{ now time.Time }

func (clock fixedHistoryClock) Now() time.Time { return clock.now }

type fakeHistoryRepositoryReader struct {
	repository *config.ReleaseRepository
	err        error
	root       string
	calls      int
}

func (reader *fakeHistoryRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	reader.calls++
	reader.root = root
	return reader.repository, reader.err
}

type historyCountCall struct {
	from  string
	to    string
	paths []string
}

type fakeHistoryGitReader struct {
	unitCountErr     map[int]error
	unitTagsErr      error
	spec             config.TagSpec
	legacyTags       []string
	legacyCounts     []int
	unitTags         []historyUnitTag
	unitCounts       []int
	legacyCountCalls []historyCountCall
	countCalls       []historyCountCall
	legacyTagsCalls  int
	unitTagsCalls    int
}

func (reader *fakeHistoryGitReader) LegacyTags() []string {
	reader.legacyTagsCalls++
	return reader.legacyTags
}

func (reader *fakeHistoryGitReader) LegacyCommitCount(from, to string) int {
	call := historyCountCall{from: from, to: to}
	reader.legacyCountCalls = append(reader.legacyCountCalls, call)
	index := len(reader.legacyCountCalls) - 1
	return reader.legacyCounts[index]
}

func (reader *fakeHistoryGitReader) UnitTags(spec config.TagSpec) ([]historyUnitTag, error) {
	reader.unitTagsCalls++
	reader.spec = spec
	return reader.unitTags, reader.unitTagsErr
}

func (reader *fakeHistoryGitReader) UnitCommitCount(from, to string, paths []string) (int, error) {
	call := historyCountCall{from: from, to: to, paths: append([]string(nil), paths...)}
	reader.countCalls = append(reader.countCalls, call)
	index := len(reader.countCalls) - 1
	if err := reader.unitCountErr[index]; err != nil {
		return 0, err
	}
	return reader.unitCounts[index], nil
}

func (reader *fakeHistoryGitReader) totalCalls() int {
	return reader.legacyTagsCalls + len(reader.legacyCountCalls) + reader.unitTagsCalls + len(reader.countCalls)
}

func historyV2Repository() *config.ReleaseRepository {
	return &config.ReleaseRepository{
		SourceFormat: config.SourceFormatV2,
		Units: []config.ReleaseUnit{
			{ID: "api", Paths: []string{"api/**"}, TagPrefix: "api/v"},
			{ID: "web", Paths: []string{"web/**"}, TagPrefix: "web/v"},
		},
	}
}

func assertHistoryQueryFailure(t *testing.T, failure *historyQueryFailure, code string) {
	t.Helper()
	if failure == nil || failure.Code != code {
		t.Fatalf("failure = %#v, want %s", failure, code)
	}
}
