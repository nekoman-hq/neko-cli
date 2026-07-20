//nolint:staticcheck // Migration paths intentionally include the deprecated V1 file.
package migrate

import (
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

const (
	backupFileName  = ".release.neko.json.v1.bak"
	journalFileName = "release.migration.json"
)

type migrationPathSet struct {
	source       string
	config       string
	state        string
	backup       string
	journal      string
	pairRecovery string
}

func migrationPaths(root string) migrationPathSet {
	return migrationPathSet{
		source:       filepath.Join(root, releaseconfig.V1FileName),
		config:       releaseconfig.V2ConfigPath(root),
		state:        releaseconfig.V2StatePath(root),
		backup:       filepath.Join(root, backupFileName),
		journal:      filepath.Join(root, releaseconfig.V2Directory, journalFileName),
		pairRecovery: releaseconfig.V2PairRecoveryPath(root),
	}
}

func captureMigrationFile(path string) (migrationFileSnapshot, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return migrationFileSnapshot{path: path}, nil
	}
	if err != nil {
		return migrationFileSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return migrationFileSnapshot{}, err
	}
	return newMigrationFileSnapshot(path, data, info.Mode().Perm()), nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
