package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// ReleaseExecutionJournalStore stores execution journals below the Git common
// directory, outside the repository worktree.
//
//nolint:govet // Fields are grouped by construction dependency, not memory layout.
type ReleaseExecutionJournalStore struct {
	RepositoryRoot string
	runner         gitCommandRunner
	now            func() time.Time
}

//nolint:govet // Resolution fields follow caller decision order.
type ReleaseExecutionJournalResolution struct {
	Path     string
	Journal  *ReleaseExecutionJournal
	Created  bool
	Reused   bool
	Guidance string
}

func NewReleaseExecutionJournalStore(repositoryRoot string) *ReleaseExecutionJournalStore {
	return &ReleaseExecutionJournalStore{
		RepositoryRoot: repositoryRoot,
		runner:         execGitRunner{},
		now:            func() time.Time { return time.Now().UTC() },
	}
}

func (store *ReleaseExecutionJournalStore) JournalPath(identity ReleaseExecutionIdentity) (string, error) {
	if !isSafeDispatchIdentityHash(identity.SHA256) {
		return "", fmt.Errorf("release execution identity hash %q is not safe", identity.SHA256)
	}
	commonDir, err := store.gitCommonDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(commonDir, "neko", "release", "executions", identity.SHA256+".json"), nil
}

func (store *ReleaseExecutionJournalStore) Prepare(expected *ReleaseExecutionJournal) (*ReleaseExecutionJournalResolution, error) {
	if expected == nil {
		return nil, fmt.Errorf("release execution journal is missing")
	}
	path, err := store.JournalPath(expected.Identity)
	if err != nil {
		return nil, err
	}
	existing, err := store.loadAt(path)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := existing.ValidateImmutable(expected); err != nil {
			return nil, fmt.Errorf("existing release execution journal %s conflicts with intent: %w", path, err)
		}
		return &ReleaseExecutionJournalResolution{
			Path:     path,
			Journal:  existing,
			Reused:   true,
			Guidance: "Existing release execution journal matches the intended transaction.",
		}, nil
	}
	copy := *expected
	copy.CreatedAt = store.now()
	copy.UpdatedAt = copy.CreatedAt
	if err := store.writeAtomic(path, &copy); err != nil {
		return nil, err
	}
	return &ReleaseExecutionJournalResolution{
		Path:     path,
		Journal:  &copy,
		Created:  true,
		Guidance: "Prepared release execution journal created. No release mutation has been performed by the store.",
	}, nil
}

func (store *ReleaseExecutionJournalStore) Load(identity ReleaseExecutionIdentity) (*ReleaseExecutionJournalResolution, error) {
	path, err := store.JournalPath(identity)
	if err != nil {
		return nil, err
	}
	journal, err := store.loadAt(path)
	if err != nil {
		return nil, err
	}
	if journal == nil {
		return &ReleaseExecutionJournalResolution{
			Path:     path,
			Guidance: "No release execution journal exists for this identity.",
		}, nil
	}
	return &ReleaseExecutionJournalResolution{
		Path:     path,
		Journal:  journal,
		Reused:   true,
		Guidance: "Release execution journal loaded.",
	}, nil
}

func (store *ReleaseExecutionJournalStore) FindUnresolved(repositoryRemote, unitID string) ([]ReleaseExecutionJournalResolution, error) {
	commonDir, err := store.gitCommonDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(commonDir, "neko", "release", "executions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read release execution journal directory %s: %w", dir, err)
	}
	var matches []ReleaseExecutionJournalResolution
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		journal, err := store.loadAt(path)
		if err != nil {
			return nil, err
		}
		if journal == nil || journal.State == ReleaseExecutionHandoffReady {
			continue
		}
		if journal.RepositoryRemote == strings.TrimSpace(repositoryRemote) && journal.UnitID == unitID {
			matches = append(matches, ReleaseExecutionJournalResolution{
				Path:    path,
				Journal: journal,
				Reused:  true,
			})
		}
	}
	return matches, nil
}

// BeginPending atomically persists a pending mutation marker before the caller
// performs that mutation.
func (store *ReleaseExecutionJournalStore) BeginPending(identity ReleaseExecutionIdentity, action ReleaseExecutionPendingAction) (*ReleaseExecutionJournalResolution, error) {
	path, journal, err := store.loadExisting(identity)
	if err != nil {
		return nil, err
	}
	if err := journal.BeginPending(action, store.now()); err != nil {
		return nil, err
	}
	if err := store.writeAtomic(path, journal); err != nil {
		return nil, err
	}
	return &ReleaseExecutionJournalResolution{Path: path, Journal: journal, Reused: true}, nil
}

// ConfirmPhase atomically records a confirmed phase and clears the matching
// pending marker after the caller has successfully completed the mutation.
func (store *ReleaseExecutionJournalStore) ConfirmPhase(identity ReleaseExecutionIdentity, next ReleaseExecutionJournalState, update ReleaseExecutionJournalUpdate) (*ReleaseExecutionJournalResolution, error) {
	path, journal, err := store.loadExisting(identity)
	if err != nil {
		return nil, err
	}
	if err := journal.ConfirmPhase(next, update, store.now()); err != nil {
		return nil, err
	}
	if err := store.writeAtomic(path, journal); err != nil {
		return nil, err
	}
	return &ReleaseExecutionJournalResolution{Path: path, Journal: journal, Reused: true}, nil
}

func (store *ReleaseExecutionJournalStore) RecordLastError(identity ReleaseExecutionIdentity, message string) (*ReleaseExecutionJournalResolution, error) {
	path, journal, err := store.loadExisting(identity)
	if err != nil {
		return nil, err
	}
	journal.LastError = capDispatchText(message)
	journal.touch(store.now())
	if err := store.writeAtomic(path, journal); err != nil {
		return nil, err
	}
	return &ReleaseExecutionJournalResolution{Path: path, Journal: journal, Reused: true}, nil
}

func (store *ReleaseExecutionJournalStore) loadExisting(identity ReleaseExecutionIdentity) (string, *ReleaseExecutionJournal, error) {
	path, err := store.JournalPath(identity)
	if err != nil {
		return "", nil, err
	}
	journal, err := store.loadAt(path)
	if err != nil {
		return "", nil, err
	}
	if journal == nil {
		return "", nil, fmt.Errorf("release execution journal %s is missing", path)
	}
	if journal.Identity.SHA256 != identity.SHA256 {
		return "", nil, fmt.Errorf("release execution journal %s identity mismatch", path)
	}
	return path, journal, nil
}

func (store *ReleaseExecutionJournalStore) gitCommonDir() (string, error) {
	output, err := store.runner.Run(store.RepositoryRoot, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("resolve git common dir: %w", err)
	}
	commonDir := strings.TrimSpace(output)
	if commonDir == "" {
		return "", fmt.Errorf("git common dir is empty")
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(store.RepositoryRoot, commonDir)
	}
	absolute, err := filepath.Abs(commonDir)
	if err != nil {
		return "", fmt.Errorf("resolve git common dir %q: %w", commonDir, err)
	}
	return absolute, nil
}

func (store *ReleaseExecutionJournalStore) loadAt(path string) (*ReleaseExecutionJournal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read release execution journal %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var journal ReleaseExecutionJournal
	if err := decoder.Decode(&journal); err != nil {
		return nil, fmt.Errorf("parse release execution journal %s: %w", path, err)
	}
	if journal.SchemaVersion != releaseExecutionJournalSchemaVersion {
		return nil, fmt.Errorf("release execution journal %s schemaVersion mismatch", path)
	}
	if !journal.State.Valid() {
		return nil, fmt.Errorf("release execution journal %s has invalid state %q", path, journal.State)
	}
	if !journal.PendingAction.Valid() {
		return nil, fmt.Errorf("release execution journal %s has invalid pending action %q", path, journal.PendingAction)
	}
	return &journal, nil
}

func (store *ReleaseExecutionJournalStore) writeAtomic(path string, journal *ReleaseExecutionJournal) error {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal release execution journal: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create release execution journal directory %s: %w", filepath.Dir(path), err)
	}
	if err := releaseconfig.AtomicWriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write release execution journal %s: %w", path, err)
	}
	return nil
}
