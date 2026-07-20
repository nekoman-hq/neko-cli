package validate

import "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"

type validationReleaseRepositoryReader struct{}

func (validationReleaseRepositoryReader) Read(root string) (*config.ReleaseRepository, bool, error) {
	repository, err := config.LoadReleaseRepository(root)
	if err == nil {
		return repository, true, nil
	}
	present := config.V2ConfigExists(root) || config.V1ConfigExistsAt(root)
	return nil, present, err
}
