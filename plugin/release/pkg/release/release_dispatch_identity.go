package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ReleaseDispatchIdentity is the stable hash input for one future dispatch
// attempt. It intentionally uses the Git remote identity rather than the local
// checkout path so worktrees and clones of the same remote resolve to the same
// release identity.
//
//nolint:govet // Identity fields are ordered by canonical domain meaning.
type ReleaseDispatchIdentity struct {
	RepositoryRemoteName string `json:"repositoryRemoteName"`
	RepositoryRemote     string `json:"repositoryRemote"`
	UnitID               string `json:"unit"`
	Version              string `json:"version"`
	Tag                  string `json:"tag"`
	ReleaseCommitSHA     string `json:"releaseCommitSHA"`
	WorkflowPath         string `json:"workflowPath"`
	Executor             string `json:"executor"`
	Delivery             string `json:"delivery"`
	SHA256               string `json:"sha256"`
}

func newReleaseDispatchIdentity(repositoryRemoteName, repositoryRemote, unitID, version, tag, releaseCommitSHA, workflowPath, executor, delivery string) (ReleaseDispatchIdentity, error) {
	identity := ReleaseDispatchIdentity{
		RepositoryRemoteName: strings.TrimSpace(repositoryRemoteName),
		RepositoryRemote:     strings.TrimSpace(repositoryRemote),
		UnitID:               unitID,
		Version:              version,
		Tag:                  tag,
		ReleaseCommitSHA:     releaseCommitSHA,
		WorkflowPath:         workflowPath,
		Executor:             executor,
		Delivery:             delivery,
	}
	if identity.RepositoryRemote == "" {
		return ReleaseDispatchIdentity{}, fmt.Errorf("repository remote identity is missing")
	}
	if identity.RepositoryRemoteName == "" {
		return ReleaseDispatchIdentity{}, fmt.Errorf("repository remote name is missing")
	}
	hash, err := hashDispatchIdentity(identity)
	if err != nil {
		return ReleaseDispatchIdentity{}, err
	}
	identity.SHA256 = hash
	return identity, nil
}

func hashDispatchIdentity(identity ReleaseDispatchIdentity) (string, error) {
	canonical := struct {
		RepositoryRemoteName string `json:"repositoryRemoteName"`
		RepositoryRemote     string `json:"repositoryRemote"`
		UnitID               string `json:"unit"`
		Version              string `json:"version"`
		Tag                  string `json:"tag"`
		ReleaseCommitSHA     string `json:"releaseCommitSHA"`
		WorkflowPath         string `json:"workflowPath"`
		Executor             string `json:"executor"`
		Delivery             string `json:"delivery"`
	}{
		RepositoryRemoteName: identity.RepositoryRemoteName,
		RepositoryRemote:     identity.RepositoryRemote,
		UnitID:               identity.UnitID,
		Version:              identity.Version,
		Tag:                  identity.Tag,
		ReleaseCommitSHA:     identity.ReleaseCommitSHA,
		WorkflowPath:         identity.WorkflowPath,
		Executor:             identity.Executor,
		Delivery:             identity.Delivery,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal dispatch identity: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
