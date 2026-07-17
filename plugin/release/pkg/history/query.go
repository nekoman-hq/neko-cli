package history

import (
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
)

type historyQueryRequest struct {
	Unit string
}

type historyQueryEntry struct {
	Unit    string
	Version string
	From    string
	Commits int
}

type historyQueryResult struct {
	SourceFormat config.SourceFormat
	Entries      []historyQueryEntry
}

type historyQueryFailure struct {
	Cause   error
	Code    string
	Message string
}

type historyUnitTag struct {
	Tag string
}

type historyQuerier interface {
	Query(historyQueryRequest) (historyQueryResult, *historyQueryFailure)
}

type historyRepositoryReader interface {
	Load(string) (*config.ReleaseRepository, error)
}

type historyGitReader interface {
	LegacyTags() []string
	LegacyCommitCount(from, to string) int
	UnitTags(config.TagSpec) ([]historyUnitTag, error)
	UnitCommitCount(from, to string, paths []string) (int, error)
}

type historyQueryUseCase struct {
	repositoryRoot string
	repositories   historyRepositoryReader
	git            historyGitReader
}

func newHistoryQueryUseCase(repositories historyRepositoryReader, gitReader historyGitReader) historyQueryUseCase {
	return newHistoryQueryUseCaseAt(".", repositories, gitReader)
}

func newHistoryQueryUseCaseAt(root string, repositories historyRepositoryReader, gitReader historyGitReader) historyQueryUseCase {
	return historyQueryUseCase{repositoryRoot: root, repositories: repositories, git: gitReader}
}

func (useCase historyQueryUseCase) Query(request historyQueryRequest) (historyQueryResult, *historyQueryFailure) {
	repository, err := useCase.repositories.Load(useCase.repositoryRoot)
	if err != nil {
		return historyQueryResult{}, historyFailure("CONFIG_INVALID", err)
	}
	unit, err := config.ResolveReleaseUnit(repository, request.Unit, config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return historyQueryResult{}, historyFailure("UNIT_RESOLUTION_FAILED", err)
	}
	if repository.SourceFormat == config.SourceFormatV2 {
		return useCase.queryUnitHistory(*unit)
	}
	return useCase.queryLegacyHistory(), nil
}

func (useCase historyQueryUseCase) queryLegacyHistory() historyQueryResult {
	tags := useCase.git.LegacyTags()

	entries := make([]historyQueryEntry, 0, len(tags))
	for i, tag := range tags {
		from := ""
		if i > 0 {
			from = tags[i-1]
		}
		entries = append(entries, historyQueryEntry{
			Version: tag,
			From:    from,
			Commits: useCase.git.LegacyCommitCount(from, tag),
		})
	}
	return historyQueryResult{SourceFormat: config.SourceFormatV1, Entries: entries}
}

func (useCase historyQueryUseCase) queryUnitHistory(unit config.ReleaseUnit) (historyQueryResult, *historyQueryFailure) {
	spec, err := config.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return historyQueryResult{}, historyFailure("TAG_SPEC_INVALID", err)
	}
	tags, err := useCase.git.UnitTags(spec)
	if err != nil {
		return historyQueryResult{}, historyFailure("GIT_HISTORY_FAILED", err)
	}

	entries := make([]historyQueryEntry, 0, len(tags))
	for i, tag := range tags {
		from := ""
		if i > 0 {
			from = tags[i-1].Tag
		}
		commitCount, err := useCase.git.UnitCommitCount(from, tag.Tag, unit.Paths)
		if err != nil {
			return historyQueryResult{}, historyFailure("GIT_HISTORY_FAILED", err)
		}
		entries = append(entries, historyQueryEntry{
			Unit:    unit.ID,
			Version: tag.Tag,
			From:    from,
			Commits: commitCount,
		})
	}
	return historyQueryResult{SourceFormat: config.SourceFormatV2, Entries: entries}, nil
}

func historyFailure(code string, err error) *historyQueryFailure {
	return &historyQueryFailure{Cause: err, Code: code, Message: err.Error()}
}

type historyReleaseRepositoryReader struct{}

func (historyReleaseRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	return config.LoadReleaseRepository(root)
}

type historyGitAdapter struct {
	repositoryRoot string
}

func (adapter historyGitAdapter) LegacyTags() []string {
	return git.GetTagsAt(adapter.repositoryRoot)
}

func (adapter historyGitAdapter) LegacyCommitCount(from, to string) int {
	return git.CountCommitsBetweenAt(adapter.repositoryRoot, from, to)
}

func (adapter historyGitAdapter) UnitTags(spec config.TagSpec) ([]historyUnitTag, error) {
	tags, err := git.UnitTagsInHistoryAt(adapter.repositoryRoot, spec)
	entries := make([]historyUnitTag, 0, len(tags))
	for _, tag := range tags {
		entries = append(entries, historyUnitTag{Tag: tag.Tag})
	}
	return entries, err
}

func (adapter historyGitAdapter) UnitCommitCount(from, to string, paths []string) (int, error) {
	return git.CountCommitsBetweenPathsAt(adapter.repositoryRoot, from, to, paths)
}
