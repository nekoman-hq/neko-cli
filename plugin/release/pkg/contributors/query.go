package contributors

import (
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
)

type contributorsQueryRequest struct {
	Unit string
}

type contributorQueryEntry struct {
	Author  string
	Commits string
}

type contributorsQueryResult struct {
	Contributors []contributorQueryEntry
}

type contributorsQueryFailure struct {
	Cause   error
	Code    string
	Message string
}

type contributorsQuerier interface {
	Query(contributorsQueryRequest) (contributorsQueryResult, *contributorsQueryFailure)
}

type contributorsRepositoryReader interface {
	Load(string) (*config.ReleaseRepository, error)
}

type contributorsGitReader interface {
	ForRepository() ([]contributorQueryEntry, error)
	ForPaths([]string) ([]contributorQueryEntry, error)
}

type contributorsQueryUseCase struct {
	repositoryRoot string
	repositories   contributorsRepositoryReader
	git            contributorsGitReader
}

func newContributorsQueryUseCase(repositories contributorsRepositoryReader, gitReader contributorsGitReader) contributorsQueryUseCase {
	return newContributorsQueryUseCaseAt(".", repositories, gitReader)
}

func newContributorsQueryUseCaseAt(root string, repositories contributorsRepositoryReader, gitReader contributorsGitReader) contributorsQueryUseCase {
	return contributorsQueryUseCase{repositoryRoot: root, repositories: repositories, git: gitReader}
}

func (useCase contributorsQueryUseCase) Query(request contributorsQueryRequest) (contributorsQueryResult, *contributorsQueryFailure) {
	repository, err := useCase.repositories.Load(useCase.repositoryRoot)
	if err != nil {
		return contributorsQueryResult{}, contributorsFailure("CONFIG_INVALID", err)
	}
	unit, err := config.ResolveReleaseUnit(repository, request.Unit, config.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return contributorsQueryResult{}, contributorsFailure("UNIT_RESOLUTION_FAILED", err)
	}

	var entries []contributorQueryEntry
	if repository.SourceFormat == config.SourceFormatV2 {
		entries, err = useCase.git.ForPaths(unit.Paths)
	} else {
		entries, err = useCase.git.ForRepository()
	}
	if err != nil {
		return contributorsQueryResult{}, contributorsFailure("GIT_CONTRIBUTORS_FAILED", err)
	}

	return contributorsQueryResult{Contributors: append([]contributorQueryEntry(nil), entries...)}, nil
}

func contributorsFailure(code string, err error) *contributorsQueryFailure {
	return &contributorsQueryFailure{Cause: err, Code: code, Message: err.Error()}
}

type contributorsReleaseRepositoryReader struct{}

func (contributorsReleaseRepositoryReader) Load(root string) (*config.ReleaseRepository, error) {
	return config.LoadReleaseRepository(root)
}

type contributorsGitAdapter struct {
	repositoryRoot string
}

func (adapter contributorsGitAdapter) ForRepository() ([]contributorQueryEntry, error) {
	contributors, err := git.ContributorsAt(adapter.repositoryRoot)
	return contributorEntries(contributors), err
}

func (adapter contributorsGitAdapter) ForPaths(paths []string) ([]contributorQueryEntry, error) {
	contributors, err := git.ContributorsForPathsAt(adapter.repositoryRoot, paths)
	return contributorEntries(contributors), err
}

func contributorEntries(contributors []git.Contributor) []contributorQueryEntry {
	entries := make([]contributorQueryEntry, 0, len(contributors))
	for _, contributor := range contributors {
		entries = append(entries, contributorQueryEntry{
			Author:  contributor.Author,
			Commits: contributor.Commits,
		})
	}
	return entries
}
