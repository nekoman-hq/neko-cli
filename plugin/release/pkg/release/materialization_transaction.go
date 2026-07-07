package release

import (
	"fmt"
	"os"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type MaterializationSnapshot struct {
	Path    string
	Bytes   []byte
	Mode    os.FileMode
	Existed bool
}

type AppliedMaterialization struct {
	ChangedFiles  []string
	RestoredFiles []string
}

type MaterializationTransaction struct {
	Plan      *MaterializationPlan
	Snapshots map[string]MaterializationSnapshot
	Applied   []MaterializedFileChange
}

func NewMaterializationTransaction(plan *MaterializationPlan) *MaterializationTransaction {
	return &MaterializationTransaction{
		Plan:      plan,
		Snapshots: make(map[string]MaterializationSnapshot),
	}
}

func (mt *MaterializationTransaction) CaptureSnapshots() error {
	if err := ValidateMaterializationPlan(mt.Plan); err != nil {
		return err
	}
	for _, change := range mt.Plan.Changes {
		data, mode, existed, err := readMaterializedFile(change.AbsolutePath)
		if err != nil {
			return err
		}
		mt.Snapshots[change.AbsolutePath] = MaterializationSnapshot{
			Path:    change.AbsolutePath,
			Bytes:   append([]byte(nil), data...),
			Mode:    mode,
			Existed: existed,
		}
	}
	return nil
}

func (mt *MaterializationTransaction) Apply() (*AppliedMaterialization, error) {
	result := &AppliedMaterialization{}
	for _, change := range mt.Plan.Changes {
		mode := change.FileMode
		if mode == 0 {
			mode = 0644
		}
		if err := releaseconfig.AtomicWriteFile(change.AbsolutePath, change.AfterContent, mode); err != nil {
			if restoreErr := mt.Restore(); restoreErr != nil {
				return result, fmt.Errorf("%w; failed to restore materialized files: %v", err, restoreErr)
			}
			return result, err
		}
		mt.Applied = append(mt.Applied, change)
		result.ChangedFiles = append(result.ChangedFiles, change.AbsolutePath)
	}
	return result, nil
}

func (mt *MaterializationTransaction) Restore() error {
	var restored []string
	for i := len(mt.Applied) - 1; i >= 0; i-- {
		change := mt.Applied[i]
		snapshot, ok := mt.Snapshots[change.AbsolutePath]
		if !ok {
			return fmt.Errorf("materialization snapshot missing for %s", change.AbsolutePath)
		}
		if !snapshot.Existed {
			if err := os.Remove(snapshot.Path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove materialized file %s: %w", snapshot.Path, err)
			}
			restored = append(restored, snapshot.Path)
			continue
		}
		if err := releaseconfig.AtomicWriteFile(snapshot.Path, snapshot.Bytes, snapshot.Mode); err != nil {
			return fmt.Errorf("restore materialized file %s: %w", snapshot.Path, err)
		}
		restored = append(restored, snapshot.Path)
	}
	mt.Applied = nil
	_ = restored
	return nil
}

func (mt *MaterializationTransaction) ChangedFiles() []string {
	files := make([]string, 0, len(mt.Plan.Changes))
	for _, change := range mt.Plan.Changes {
		files = append(files, change.AbsolutePath)
	}
	return files
}

func (mt *MaterializationTransaction) RequiredCommitFiles() []MaterializedFileChange {
	var files []MaterializedFileChange
	for _, change := range mt.Plan.Changes {
		if change.RequiredForReleaseCommit {
			files = append(files, change)
		}
	}
	return files
}
