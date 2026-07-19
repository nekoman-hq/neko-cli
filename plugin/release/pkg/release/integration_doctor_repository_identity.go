package release

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type integrationDoctorRepositoryIdentity struct {
	Owner      string
	Repository string
	RemoteURL  string
}

func (identity integrationDoctorRepositoryIdentity) Name() string {
	if identity.Owner == "" || identity.Repository == "" {
		return ""
	}
	return identity.Owner + "/" + identity.Repository
}

type integrationDoctorRepositoryIdentityReader interface {
	ReadOrigin(string) (integrationDoctorRepositoryIdentity, error)
}

type filesystemIntegrationDoctorRepositoryIdentityReader struct{}

func (filesystemIntegrationDoctorRepositoryIdentityReader) ReadOrigin(
	repositoryRoot string,
) (integrationDoctorRepositoryIdentity, error) {
	configPath, err := integrationDoctorGitConfigPath(repositoryRoot)
	if err != nil {
		return integrationDoctorRepositoryIdentity{}, err
	}
	content, err := os.ReadFile(configPath)
	if err != nil {
		return integrationDoctorRepositoryIdentity{}, fmt.Errorf("read local git configuration: %w", err)
	}
	remoteURL := integrationDoctorOriginURL(content)
	if remoteURL == "" {
		return integrationDoctorRepositoryIdentity{}, fmt.Errorf("local git configuration has no origin URL")
	}
	target, err := ResolveGitHubRepositoryTarget("origin", remoteURL)
	if err != nil {
		return integrationDoctorRepositoryIdentity{}, fmt.Errorf("resolve local origin identity: %w", err)
	}
	return integrationDoctorRepositoryIdentity{
		Owner: target.Owner, Repository: target.Repository, RemoteURL: remoteURL,
	}, nil
}

func integrationDoctorGitConfigPath(repositoryRoot string) (string, error) {
	gitMarker := filepath.Join(repositoryRoot, ".git")
	info, err := os.Lstat(gitMarker)
	if err != nil {
		return "", fmt.Errorf("inspect local git directory: %w", err)
	}
	gitDirectory := gitMarker
	if info.Mode().IsRegular() {
		content, readErr := os.ReadFile(gitMarker)
		if readErr != nil {
			return "", fmt.Errorf("read local git directory marker: %w", readErr)
		}
		line := strings.TrimSpace(string(content))
		if !strings.HasPrefix(line, "gitdir: ") {
			return "", fmt.Errorf("local git directory marker is malformed")
		}
		gitDirectory = strings.TrimSpace(strings.TrimPrefix(line, "gitdir: "))
		if !filepath.IsAbs(gitDirectory) {
			gitDirectory = filepath.Join(repositoryRoot, gitDirectory)
		}
	}
	configPath := filepath.Join(gitDirectory, "config")
	if _, statErr := os.Stat(configPath); statErr == nil {
		return configPath, nil
	}
	commonContent, err := os.ReadFile(filepath.Join(gitDirectory, "commondir"))
	if err != nil {
		return "", fmt.Errorf("locate local git configuration: %w", err)
	}
	commonDirectory := strings.TrimSpace(string(commonContent))
	if !filepath.IsAbs(commonDirectory) {
		commonDirectory = filepath.Join(gitDirectory, commonDirectory)
	}
	return filepath.Join(commonDirectory, "config"), nil
}

func integrationDoctorOriginURL(content []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	inOrigin := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if !inOrigin {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(key) == "url" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

var _ integrationDoctorRepositoryIdentityReader = filesystemIntegrationDoctorRepositoryIdentityReader{}
