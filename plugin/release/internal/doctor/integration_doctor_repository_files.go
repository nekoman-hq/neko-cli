package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type integrationDoctorRepositoryFileReader interface {
	ReadFile(repositoryRoot, relativePath string) ([]byte, error)
}

type filesystemIntegrationDoctorRepositoryFileReader struct{}

func (filesystemIntegrationDoctorRepositoryFileReader) ReadFile(
	repositoryRoot, relativePath string,
) ([]byte, error) {
	if filepath.IsAbs(relativePath) || strings.HasPrefix(relativePath, "/") || strings.Contains(relativePath, `\`) {
		return nil, fmt.Errorf("repository evidence path %q must be repository-relative and use forward slashes", relativePath)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relativePath)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != relativePath {
		return nil, fmt.Errorf("repository evidence path %q must be clean and repository-relative", relativePath)
	}

	absoluteRoot, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository evidence root: %w", err)
	}
	physicalRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve repository evidence root: %w", err)
	}
	target := filepath.Join(absoluteRoot, filepath.FromSlash(relativePath))
	info, err := os.Lstat(target)
	if err != nil {
		return nil, fmt.Errorf("inspect repository evidence %s: %w", relativePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("repository evidence %s must be a regular file", relativePath)
	}
	physicalTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return nil, fmt.Errorf("resolve repository evidence %s: %w", relativePath, err)
	}
	relativeTarget, err := filepath.Rel(physicalRoot, physicalTarget)
	if err != nil || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("repository evidence %s resolves outside the repository root", relativePath)
	}
	content, err := os.ReadFile(physicalTarget)
	if err != nil {
		return nil, fmt.Errorf("read repository evidence %s: %w", relativePath, err)
	}
	return content, nil
}

var _ integrationDoctorRepositoryFileReader = filesystemIntegrationDoctorRepositoryFileReader{}
