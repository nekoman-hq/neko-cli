package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// releaseJournalFiles owns only the common location and secure file mechanics
// shared by the distinct execution and dispatch journal stores.
type releaseJournalFiles struct {
	git            gitCommandRunner
	makeDirectory  func(string, os.FileMode) error
	replaceFile    func(string, []byte, os.FileMode) error
	repositoryRoot string
}

func newReleaseJournalFiles(repositoryRoot string, git gitCommandRunner) releaseJournalFiles {
	return releaseJournalFiles{
		repositoryRoot: repositoryRoot,
		git:            git,
		makeDirectory:  os.MkdirAll,
		replaceFile:    releaseconfig.AtomicWriteFile,
	}
}

func (files releaseJournalFiles) executionPath(identityHash string) (string, error) {
	directory, err := files.executionDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, identityHash+".json"), nil
}

func (files releaseJournalFiles) dispatchPath(identityHash string) (string, error) {
	directory, err := files.dispatchDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, identityHash+".json"), nil
}

func (files releaseJournalFiles) executionDirectory() (string, error) {
	releaseDirectory, err := files.releaseDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(releaseDirectory, "executions"), nil
}

func (files releaseJournalFiles) dispatchDirectory() (string, error) {
	releaseDirectory, err := files.releaseDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(releaseDirectory, "dispatches"), nil
}

func (files releaseJournalFiles) releaseDirectory() (string, error) {
	commonDir, err := files.gitCommonDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "neko", "release"), nil
}

func (files releaseJournalFiles) gitCommonDirectory() (string, error) {
	output, err := files.git.Run(files.repositoryRoot, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve git common dir: %w", err)
	}
	commonDir := strings.TrimSpace(output)
	if commonDir == "" {
		return "", fmt.Errorf("git common dir is empty")
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(files.repositoryRoot, commonDir)
	}
	absolute, err := filepath.Abs(commonDir)
	if err != nil {
		return "", fmt.Errorf("resolve git common dir %q: %w", commonDir, err)
	}
	return absolute, nil
}

func marshalCanonicalReleaseJournal(journal any) ([]byte, error) {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (files releaseJournalFiles) createPrivateDirectory(path string) error {
	return files.makeDirectory(path, 0700)
}

func (files releaseJournalFiles) writePrivateAtomic(path string, data []byte) error {
	return files.replaceFile(path, data, 0600)
}
