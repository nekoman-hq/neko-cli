//nolint:staticcheck // This file is the durable adapter boundary for deprecated V1 compatibility.
package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type v1CompensationEvidenceStores interface {
	Open(string) V1CompensationEvidenceStore
}

type systemV1CompensationEvidenceStores struct {
	git gitCommandRunner
}

func (stores systemV1CompensationEvidenceStores) Open(repositoryRoot string) V1CompensationEvidenceStore {
	return NewV1CompensationEvidenceStore(repositoryRoot, stores.git)
}

type v1CompensationEvidenceGitRunner struct {
	runner v1GitCommandRunner
}

func (runner v1CompensationEvidenceGitRunner) Run(repositoryRoot string, args ...string) (string, error) {
	output, err := runner.runner.CombinedOutput(repositoryRoot, args...)
	return string(output), err
}

type v1CompensationConfigFiles interface {
	Read(string) ([]byte, error)
	Restore(string, []byte) error
	VerifyVersion(string, string) error
}

type systemV1CompensationConfigFiles struct{}

func (systemV1CompensationConfigFiles) Read(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read original V1 config: %w", err)
	}
	return data, nil
}

func (systemV1CompensationConfigFiles) Restore(path string, original []byte) error {
	if err := releaseconfig.AtomicWriteFile(path, original, 0644); err != nil {
		return fmt.Errorf("restore original V1 config: %w", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verify original V1 config: %w", err)
	}
	if !bytes.Equal(current, original) {
		return fmt.Errorf("verify original V1 config: restored bytes differ")
	}
	return nil
}

func (systemV1CompensationConfigFiles) VerifyVersion(path, expected string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("verify V1 config version: %w", err)
	}
	var config releaseconfig.V1ReleaseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("verify V1 config version: %w", err)
	}
	if config.Version != expected {
		return fmt.Errorf("verify V1 config version: got %q, want %q", config.Version, expected)
	}
	return nil
}

// systemV1CompensationGit adds post-effect verification and idempotent local
// checks without changing the legacy direct Rollback compatibility adapter.
type systemV1CompensationGit struct {
	effects systemV1RollbackGit
	runner  v1GitCommandRunner
}

func (git systemV1CompensationGit) DeleteLocalTag(root, tag string) error {
	exists, err := git.localTagExists(root, tag)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if deleteErr := git.effects.DeleteLocalTag(root, tag); deleteErr != nil {
		return deleteErr
	}
	exists, err = git.localTagExists(root, tag)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("verify local V1 tag deletion: tag %s is still present", tag)
	}
	return nil
}

func (git systemV1CompensationGit) DeleteRemoteTag(root, tag string) error {
	if err := git.effects.DeleteRemoteTag(root, tag); err != nil {
		return err
	}
	output, err := git.runner.CombinedOutput(root, "ls-remote", "--tags", "origin", "refs/tags/"+tag)
	if err != nil {
		return fmt.Errorf("verify remote V1 tag deletion: %s: %w", strings.TrimSpace(string(output)), err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("verify remote V1 tag deletion: tag %s is still present", tag)
	}
	return nil
}

func (git systemV1CompensationGit) RevertCommit(root, hash string) error {
	if err := git.effects.RevertCommit(root, hash); err != nil {
		return err
	}
	head, err := git.shortHead(root)
	if err != nil {
		return err
	}
	if head == hash {
		return fmt.Errorf("verify V1 release commit revert: HEAD did not change")
	}
	return nil
}

func (git systemV1CompensationGit) CreateFallbackCommit(root, message string) error {
	return git.effects.CreateFallbackCommit(root, message)
}

func (git systemV1CompensationGit) PushCommits(root string) error {
	return git.effects.PushCommits(root)
}

func (git systemV1CompensationGit) HardResetTo(root, hash string) error {
	if err := git.effects.HardResetTo(root, hash); err != nil {
		return err
	}
	head, err := git.shortHead(root)
	if err != nil {
		return err
	}
	if head != hash {
		return fmt.Errorf("verify V1 release commit reset: HEAD is %s, want %s", head, hash)
	}
	return nil
}

func (git systemV1CompensationGit) CleanUntracked(root string) error {
	if err := git.effects.CleanUntracked(root); err != nil {
		return err
	}
	output, err := git.runner.CombinedOutput(root, "status", "--porcelain", "--untracked-files=only")
	if err != nil {
		return fmt.Errorf("verify V1 untracked cleanup: %s: %w", strings.TrimSpace(string(output)), err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("verify V1 untracked cleanup: untracked files remain")
	}
	return nil
}

func (git systemV1CompensationGit) localTagExists(root, tag string) (bool, error) {
	output, err := git.runner.CombinedOutput(root, "tag", "--list", tag)
	if err != nil {
		return false, fmt.Errorf("inspect local V1 tag %s: %s: %w", tag, strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func (git systemV1CompensationGit) shortHead(root string) (string, error) {
	output, err := git.runner.CombinedOutput(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("inspect V1 compensation HEAD: %s: %w", strings.TrimSpace(string(output)), err)
	}
	return strings.TrimSpace(string(output)), nil
}
