package release

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// ReleaseExecutionIdentity is the stable identity for one intended V2 release
// transaction before the final release commit exists.
//
//nolint:govet // Identity fields are ordered by release intent meaning.
type ReleaseExecutionIdentity struct {
	RepositoryRemote string `json:"repositoryRemote"`
	BaseCommitSHA    string `json:"baseCommitSHA"`
	UnitID           string `json:"unit"`
	CurrentVersion   string `json:"currentVersion"`
	NextVersion      string `json:"nextVersion"`
	Tag              string `json:"tag"`
	Executor         string `json:"executor"`
	Delivery         string `json:"delivery"`
	WorkflowPath     string `json:"workflowPath,omitempty"`
	SHA256           string `json:"sha256"`
}

func newReleaseExecutionIdentity(repositoryRemote, baseCommitSHA, unitID, currentVersion, nextVersion, tag, executor, delivery, workflowPath string) (ReleaseExecutionIdentity, error) {
	identity := ReleaseExecutionIdentity{
		RepositoryRemote: strings.TrimSpace(repositoryRemote),
		BaseCommitSHA:    strings.TrimSpace(baseCommitSHA),
		UnitID:           unitID,
		CurrentVersion:   currentVersion,
		NextVersion:      nextVersion,
		Tag:              tag,
		Executor:         executor,
		Delivery:         delivery,
		WorkflowPath:     workflowPath,
	}
	if identity.RepositoryRemote == "" {
		return ReleaseExecutionIdentity{}, fmt.Errorf("release execution identity requires repository remote identity")
	}
	if !fullGitSHARegexp.MatchString(identity.BaseCommitSHA) {
		return ReleaseExecutionIdentity{}, fmt.Errorf("release execution identity requires full base commit SHA, got %q", identity.BaseCommitSHA)
	}
	hash, err := hashReleaseExecutionIdentity(identity)
	if err != nil {
		return ReleaseExecutionIdentity{}, err
	}
	identity.SHA256 = hash
	return identity, nil
}

func hashReleaseExecutionIdentity(identity ReleaseExecutionIdentity) (string, error) {
	canonical := struct {
		RepositoryRemote string `json:"repositoryRemote"`
		BaseCommitSHA    string `json:"baseCommitSHA"`
		UnitID           string `json:"unit"`
		CurrentVersion   string `json:"currentVersion"`
		NextVersion      string `json:"nextVersion"`
		Tag              string `json:"tag"`
		Executor         string `json:"executor"`
		Delivery         string `json:"delivery"`
		WorkflowPath     string `json:"workflowPath,omitempty"`
	}{
		RepositoryRemote: identity.RepositoryRemote,
		BaseCommitSHA:    identity.BaseCommitSHA,
		UnitID:           identity.UnitID,
		CurrentVersion:   identity.CurrentVersion,
		NextVersion:      identity.NextVersion,
		Tag:              identity.Tag,
		Executor:         identity.Executor,
		Delivery:         identity.Delivery,
		WorkflowPath:     identity.WorkflowPath,
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", fmt.Errorf("marshal release execution identity: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
