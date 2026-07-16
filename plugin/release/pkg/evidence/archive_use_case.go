package evidence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type evidenceArchiveResult struct {
	Family       string
	Identity     string
	DigestSHA256 string
	SourcePath   string
	ArchivePath  string
}

type evidenceArchiveUseCase struct {
	query evidenceQueryRunner
}

func newEvidenceArchiveUseCase() evidenceArchiveUseCase {
	return evidenceArchiveUseCase{query: newEvidenceQueryUseCase()}
}

func (useCase evidenceArchiveUseCase) Archive(ctx context.Context, request evidenceArchiveRequest) (evidenceArchiveResult, error) {
	result, err := useCase.query.Query(ctx, evidenceQueryRequest{
		RepositoryRoot: request.RepositoryRoot,
		Family:         request.Family,
	})
	if err != nil {
		return evidenceArchiveResult{}, err
	}
	record, err := selectArchivableEvidenceRecord(result, request)
	if err != nil {
		return evidenceArchiveResult{}, err
	}
	source, archive, err := archiveCompletedEvidence(record)
	if err != nil {
		return evidenceArchiveResult{}, err
	}
	return evidenceArchiveResult{
		Family:       record.Family,
		Identity:     record.Identity,
		DigestSHA256: record.DigestSHA256,
		SourcePath:   source,
		ArchivePath:  archive,
	}, nil
}

func selectArchivableEvidenceRecord(result evidenceQueryResult, request evidenceArchiveRequest) (EvidenceRecord, error) {
	for _, record := range result.Records {
		if record.Family != request.Family || record.Identity != request.Identity {
			continue
		}
		if record.DigestSHA256 != request.DigestSHA256 {
			return EvidenceRecord{}, fmt.Errorf("evidence digest changed since inspection")
		}
		if !record.LifecycleAllowed || record.LifecycleOperation != "archive-completed" || record.Classification != ClassificationCompleted {
			return EvidenceRecord{}, fmt.Errorf("evidence is not eligible for archive-completed")
		}
		return record, nil
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Family == request.Family {
			return EvidenceRecord{}, fmt.Errorf("evidence family has unresolved diagnostics and cannot be archived")
		}
	}
	return EvidenceRecord{}, fmt.Errorf("matching evidence record was not found")
}

func archiveCompletedEvidence(record EvidenceRecord) (string, string, error) {
	info, err := os.Lstat(record.Path)
	if err != nil {
		return "", "", fmt.Errorf("re-observe evidence file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", "", fmt.Errorf("evidence path is not a regular file")
	}
	data, err := os.ReadFile(record.Path)
	if err != nil {
		return "", "", fmt.Errorf("read evidence file: %w", err)
	}
	if sha256Hex(data) != record.DigestSHA256 {
		return "", "", fmt.Errorf("evidence digest changed before archival")
	}
	archiveDir := filepath.Join(filepath.Dir(record.Path), "archived")
	if mkdirErr := os.MkdirAll(archiveDir, 0700); mkdirErr != nil {
		return "", "", fmt.Errorf("create evidence archive directory: %w", mkdirErr)
	}
	if chmodErr := os.Chmod(archiveDir, 0700); chmodErr != nil {
		return "", "", fmt.Errorf("secure evidence archive directory: %w", chmodErr)
	}
	archivePath := filepath.Join(archiveDir, record.Identity+"-"+record.DigestSHA256+".json")
	if _, statErr := os.Lstat(archivePath); statErr == nil {
		return "", "", fmt.Errorf("evidence archive already exists")
	} else if !os.IsNotExist(statErr) {
		return "", "", fmt.Errorf("inspect evidence archive path: %w", statErr)
	}
	if writeErr := releaseconfig.AtomicWriteFile(archivePath, data, 0600); writeErr != nil {
		return "", "", fmt.Errorf("write evidence archive: %w", writeErr)
	}
	archived, err := os.ReadFile(archivePath)
	if err != nil {
		return "", "", fmt.Errorf("verify evidence archive: %w", err)
	}
	if string(archived) != string(data) {
		return "", "", fmt.Errorf("evidence archive bytes do not match source")
	}
	if removeErr := os.Remove(record.Path); removeErr != nil {
		return "", "", fmt.Errorf("remove archived evidence source: %w", removeErr)
	}
	return record.Path, archivePath, nil
}
