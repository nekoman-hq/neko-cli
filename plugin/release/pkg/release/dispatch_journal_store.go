package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//nolint:govet // Store fields keep construction dependencies in logical order.
type DispatchJournalStore struct {
	RepositoryRoot string
	files          releaseJournalFiles
	clock          ReleaseClock
}

//nolint:govet // Resolution fields follow user-facing outcome order.
type DispatchJournalResolution struct {
	Path             string
	Journal          *DispatchJournal
	Created          bool
	Reused           bool
	Blocked          bool
	RecoveryGuidance string
}

func NewDispatchJournalStore(repositoryRoot string) *DispatchJournalStore {
	return newDispatchJournalStore(repositoryRoot, execGitRunner{}, systemReleaseClock{})
}

func newDispatchJournalStore(repositoryRoot string, git gitCommandRunner, clock ReleaseClock) *DispatchJournalStore {
	return &DispatchJournalStore{
		RepositoryRoot: repositoryRoot,
		files:          newReleaseJournalFiles(repositoryRoot, git),
		clock:          clock,
	}
}

func (store *DispatchJournalStore) JournalPath(identity ReleaseDispatchIdentity) (string, error) {
	if !isSafeDispatchIdentityHash(identity.SHA256) {
		return "", fmt.Errorf("dispatch identity hash %q is not safe", identity.SHA256)
	}
	return store.files.dispatchPath(identity.SHA256)
}

func (store *DispatchJournalStore) JournalDirectory() (string, error) {
	return store.files.dispatchDirectory()
}

func (store *DispatchJournalStore) Prepare(request *ReleaseDispatchRequest) (*DispatchJournalResolution, error) {
	if request == nil {
		return nil, errDispatchRequestMissing()
	}
	path, err := store.JournalPath(request.Identity)
	if err != nil {
		return nil, err
	}
	existing, err := store.loadAt(path)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if validationErr := existing.ValidateForRequest(request); validationErr != nil {
			return nil, fmt.Errorf("existing dispatch journal %s conflicts with request: %w", path, validationErr)
		}
		guidance := dispatchJournalRecoveryGuidance(existing.State)
		resolution := &DispatchJournalResolution{
			Path:             path,
			Journal:          existing,
			Reused:           existing.State == DispatchJournalPrepared,
			Blocked:          existing.State != DispatchJournalPrepared,
			RecoveryGuidance: guidance,
		}
		if existing.State != DispatchJournalPrepared {
			return resolution, nil
		}
		return resolution, nil
	}
	journal, err := NewPreparedDispatchJournal(request, store.clock.Now())
	if err != nil {
		return nil, err
	}
	if err := store.writeAtomic(path, journal); err != nil {
		return nil, err
	}
	return &DispatchJournalResolution{
		Path:             path,
		Journal:          journal,
		Created:          true,
		RecoveryGuidance: journal.RecoveryGuidance,
	}, nil
}

func (store *DispatchJournalStore) Load(request *ReleaseDispatchRequest) (*DispatchJournalResolution, error) {
	if request == nil {
		return nil, errDispatchRequestMissing()
	}
	path, err := store.JournalPath(request.Identity)
	if err != nil {
		return nil, err
	}
	journal, err := store.loadAt(path)
	if err != nil {
		return nil, err
	}
	if journal == nil {
		return &DispatchJournalResolution{Path: path, RecoveryGuidance: "No dispatch journal exists yet."}, nil
	}
	if err := journal.ValidateForRequest(request); err != nil {
		return nil, fmt.Errorf("dispatch journal %s conflicts with request: %w", path, err)
	}
	return &DispatchJournalResolution{
		Path:             path,
		Journal:          journal,
		Reused:           journal.State == DispatchJournalPrepared,
		Blocked:          journal.State != DispatchJournalPrepared,
		RecoveryGuidance: dispatchJournalRecoveryGuidance(journal.State),
	}, nil
}

// Transition reloads, validates and atomically persists a dispatch journal state
// change for the immutable request identity.
func (store *DispatchJournalStore) Transition(request *ReleaseDispatchRequest, next DispatchJournalState, metadata DispatchJournalMetadata, lastError string) (*DispatchJournalResolution, error) {
	if request == nil {
		return nil, errDispatchRequestMissing()
	}
	path, err := store.JournalPath(request.Identity)
	if err != nil {
		return nil, err
	}
	journal, err := store.loadAt(path)
	if err != nil {
		return nil, err
	}
	if journal == nil {
		return nil, fmt.Errorf("dispatch journal %s is missing", path)
	}
	if err := journal.ValidateForRequest(request); err != nil {
		return nil, fmt.Errorf("dispatch journal %s conflicts with request: %w", path, err)
	}
	if err := journal.Transition(next, store.clock.Now(), lastError); err != nil {
		return nil, err
	}
	mergeDispatchJournalMetadata(&journal.DispatchMetadata, metadata)
	if err := store.writeAtomic(path, journal); err != nil {
		return nil, err
	}
	return &DispatchJournalResolution{
		Path:             path,
		Journal:          journal,
		Reused:           true,
		RecoveryGuidance: journal.RecoveryGuidance,
	}, nil
}

func (store *DispatchJournalStore) loadAt(path string) (*DispatchJournal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read dispatch journal %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var journal DispatchJournal
	if err := decoder.Decode(&journal); err != nil {
		return nil, fmt.Errorf("parse dispatch journal %s: %w", path, err)
	}
	if !journal.State.Valid() {
		return nil, fmt.Errorf("dispatch journal %s has invalid state %q", path, journal.State)
	}
	return &journal, nil
}

func (store *DispatchJournalStore) writeAtomic(path string, journal *DispatchJournal) error {
	data, err := marshalCanonicalReleaseJournal(journal)
	if err != nil {
		return fmt.Errorf("marshal dispatch journal: %w", err)
	}
	if err := store.files.createPrivateDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("create dispatch journal directory %s: %w", filepath.Dir(path), err)
	}
	if err := store.files.writePrivateAtomic(path, data); err != nil {
		return fmt.Errorf("write dispatch journal %s: %w", path, err)
	}
	return nil
}

func mergeDispatchJournalMetadata(target *DispatchJournalMetadata, source DispatchJournalMetadata) {
	if source.RunID != "" {
		target.RunID = source.RunID
	}
	if source.RunURL != "" {
		target.RunURL = source.RunURL
	}
	if source.HTMLURL != "" {
		target.HTMLURL = source.HTMLURL
	}
	if source.ResponseStatus != "" {
		target.ResponseStatus = source.ResponseStatus
	}
	if source.ResponseTimestamp != "" {
		target.ResponseTimestamp = source.ResponseTimestamp
	}
	if !source.RequestStartedAt.IsZero() {
		target.RequestStartedAt = source.RequestStartedAt
	}
	if !source.RequestFinishedAt.IsZero() {
		target.RequestFinishedAt = source.RequestFinishedAt
	}
}

func isSafeDispatchIdentityHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, r := range hash {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
