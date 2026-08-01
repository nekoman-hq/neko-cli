package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	releasecmd "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func TestReleaseEvidenceUsesConciseDescribeAndVerboseNoOpAcrossCoreTransport(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GITHUB_TOKEN", "evidence-transport-secret")
	t.Setenv("GH_TOKEN", "evidence-transport-secret")
	manifest := installReleaseReadonlyHelperPlugin(t)
	root := newReleaseEvidenceTransportRepository(t)
	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t, manifest, root, "evidence", nil, releaseReadonlyMode{},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t, manifest, root, "evidence", nil, releaseReadonlyMode{describe: true},
	)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t, manifest, root, "evidence", nil, releaseReadonlyMode{verbose: true},
	)
	combinedOutput, combinedErr := executeReleaseReadonlyCommand(
		t, manifest, root, "evidence", nil, releaseReadonlyMode{describe: true, verbose: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil || combinedErr != nil {
		t.Fatalf(
			"Evidence transport exits: default=%v describe=%v verbose=%v combined=%v",
			defaultErr, describeErr, verboseErr, combinedErr,
		)
	}
	for _, want := range []string{
		"Evidence Summary", "Evidence Inventory", "release-execution", strings.Repeat("a", 64),
		"dispatch", "terminal", "GitHub rejected the workflow dispatch request",
		"corrupt", "Preserve the file and inspect manually",
	} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("Evidence default omitted %q:\n%s", want, defaultOutput)
		}
	}
	for _, section := range []string{
		"Execution Evidence", "Dispatch Evidence", "Linkage", "Local Git Evidence",
		"Classification", "Recovery Relevance", "Limitations",
	} {
		sectionHeading := "\n" + section + "\n"
		if strings.Contains(defaultOutput, sectionHeading) || !strings.Contains(describeOutput, sectionHeading) {
			t.Fatalf(
				"Evidence describe visibility for %q is incorrect:\ndefault:\n%s\ndescribe:\n%s",
				section, defaultOutput, describeOutput,
			)
		}
	}
	if verboseOutput != defaultOutput ||
		normalizeReleaseNoOpMetadata(combinedOutput) != normalizeReleaseNoOpMetadata(describeOutput) {
		t.Fatalf(
			"Evidence verbose is not an intentional no-op:\ndefault:\n%s\nverbose:\n%s\ndescribe:\n%s\ncombined:\n%s",
			defaultOutput, verboseOutput, describeOutput, combinedOutput,
		)
	}
	for _, output := range []string{defaultOutput, describeOutput, verboseOutput, combinedOutput} {
		if strings.Contains(output, root) || strings.Contains(output, "\x1b[") ||
			strings.Contains(output, "evidence-transport-secret") {
			t.Fatalf("Evidence human output exposed path, ANSI, or credential:\n%s", output)
		}
	}
}

func TestReleaseEvidenceLegacyJSONIsInvariantAcrossGlobalModesAndFilters(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	root := newReleaseEvidenceTransportRepository(t)
	modes := []releaseReadonlyMode{
		{format: "json"},
		{format: "json", describe: true},
		{format: "json", verbose: true},
		{format: "json", describe: true, verbose: true},
	}
	var baseline releaseReadonlyPublicResponse
	for index, mode := range modes {
		output, err := executeReleaseReadonlyCommand(t, manifest, root, "evidence", nil, mode)
		if err != nil {
			t.Fatalf("Evidence JSON mode %#v: %v", mode, err)
		}
		response := decodeReleaseReadonlyPublicResponse(t, output)
		if index == 0 {
			baseline = response
		} else if response.Status != baseline.Status || !reflect.DeepEqual(response.Data, baseline.Data) {
			t.Fatalf("Evidence global mode changed legacy JSON domain:\nbaseline=%#v\nmode=%#v", baseline, response)
		}
		for _, forbidden := range []string{
			"human_table", "human_properties", "describe_only", "\x1b[", "evidence-transport-secret",
		} {
			if strings.Contains(output, forbidden) {
				t.Fatalf("Evidence JSON contains %q:\n%s", forbidden, output)
			}
		}
	}

	filtered, err := executeReleaseReadonlyCommand(
		t, manifest, root, "evidence",
		[]string{"--family", "release-execution", "--unit", "api", "--identity", "aaaaaaaa"},
		releaseReadonlyMode{format: "json"},
	)
	if err != nil {
		t.Fatalf("filtered Evidence JSON: %v", err)
	}
	data := decodeReleaseReadonlyPublicResponse(t, filtered).Data
	records, ok := data["evidence"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("filtered Evidence records = %#v", data["evidence"])
	}
}

func TestReleaseEvidenceNoEvidenceDefaultRemainsActionable(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	root := newReleaseLifecycleV2Repository(t)
	output, err := executeReleaseReadonlyCommand(t, manifest, root, "evidence", nil, releaseReadonlyMode{})
	if err != nil {
		t.Fatalf("Evidence no-evidence transport: %v", err)
	}
	for _, want := range []string{
		"Evidence Summary", "No evidence found", "No release evidence files matched the selected scope",
		"Family: none", "No release evidence files were found.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("no-evidence default omitted %q:\n%s", want, output)
		}
	}
}

func TestReleaseEvidenceArchiveTransportMovesOnlySelectedEvidenceInEveryMode(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GITHUB_TOKEN", "evidence-transport-secret")
	t.Setenv("GH_TOKEN", "evidence-transport-secret")
	manifest := installReleaseReadonlyHelperPlugin(t)
	modes := []releaseReadonlyMode{
		{},
		{describe: true},
		{verbose: true},
		{describe: true, verbose: true},
		{format: "json"},
	}
	for _, mode := range modes {
		t.Run(releaseEvidenceModeName(mode), func(t *testing.T) {
			fixture := newReleaseEvidenceArchiveTransportRepository(t)
			beforeHead := strings.TrimSpace(runReleaseReadonlyGit(t, fixture.root, "rev-parse", "HEAD"))
			beforeStatus := runReleaseReadonlyGit(t, fixture.root, "status", "--porcelain", "--untracked-files=all")
			beforeTags := runReleaseReadonlyGit(t, fixture.root, "tag", "--list")
			unrelatedBefore, err := os.ReadFile(fixture.unrelatedPath)
			if err != nil {
				t.Fatalf("read unrelated evidence: %v", err)
			}
			output, executeErr := executeReleaseReadonlyCommand(
				t,
				manifest,
				fixture.root,
				"evidence-archive",
				[]string{
					"--family", "release-execution",
					"--identity", fixture.identity,
					"--digest-sha256", fixture.digest,
					"--confirm-archive",
				},
				mode,
			)
			if executeErr != nil {
				t.Fatalf("Archive mode %#v: %v\n%s", mode, executeErr, output)
			}
			if mode.format == "json" {
				response := decodeReleaseReadonlyPublicResponse(t, output)
				if response.Status != "success" {
					t.Fatalf("Archive JSON status = %q", response.Status)
				}
			} else {
				for _, want := range []string{
					"Evidence Archive Result", fixture.identity, "Confirmed", "Matched", "Archived",
				} {
					if !strings.Contains(output, want) {
						t.Fatalf("Archive mode %#v omitted %q:\n%s", mode, want, output)
					}
				}
				if mode.describe {
					for _, want := range []string{"Archive Validation", "Guarded Write Plan", "Limitations"} {
						if !strings.Contains(output, want) {
							t.Fatalf("Archive describe omitted %q:\n%s", want, output)
						}
					}
				}
				if mode.verbose {
					for _, want := range []string{
						"Validating evidence archive request",
						"Evidence digest verification completed",
						"Exact private archive write completed",
						"Archived evidence bytes verified",
						"Selected completed evidence source removed",
						"Evidence archive operation completed",
					} {
						if !strings.Contains(output, want) {
							t.Fatalf("Archive verbose omitted %q:\n%s", want, output)
						}
					}
				}
			}
			if strings.Contains(output, fixture.root) || strings.Contains(output, "\x1b[") ||
				strings.Contains(output, fixture.digest) || strings.Contains(output, "evidence-transport-secret") {
				if mode.format != "json" {
					t.Fatalf("Archive human output exposed path, ANSI, full digest, or credential:\n%s", output)
				}
			}
			if _, statErr := os.Stat(fixture.sourcePath); !os.IsNotExist(statErr) {
				t.Fatalf("selected source still exists: %v", statErr)
			}
			archived, readErr := os.ReadFile(fixture.archivePath)
			if readErr != nil || string(archived) != string(fixture.sourceBytes) {
				t.Fatalf("archive bytes = %q, err=%v", archived, readErr)
			}
			unrelatedAfter, readErr := os.ReadFile(fixture.unrelatedPath)
			if readErr != nil || string(unrelatedAfter) != string(unrelatedBefore) {
				t.Fatalf("unrelated evidence changed: %q, err=%v", unrelatedAfter, readErr)
			}
			if after := strings.TrimSpace(runReleaseReadonlyGit(t, fixture.root, "rev-parse", "HEAD")); after != beforeHead {
				t.Fatalf("archive changed HEAD: before=%s after=%s", beforeHead, after)
			}
			if after := runReleaseReadonlyGit(t, fixture.root, "status", "--porcelain", "--untracked-files=all"); after != beforeStatus {
				t.Fatalf("archive changed repository worktree/index: before=%q after=%q", beforeStatus, after)
			}
			if after := runReleaseReadonlyGit(t, fixture.root, "tag", "--list"); after != beforeTags {
				t.Fatalf("archive changed tags: before=%q after=%q", beforeTags, after)
			}
		})
	}
}

func newReleaseEvidenceTransportRepository(t *testing.T) string {
	t.Helper()
	root := newReleaseLifecycleV2Repository(t)
	executionDir, err := releasecmd.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("execution journal directory: %v", err)
	}
	dispatchDir, err := releasecmd.NewDispatchJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("dispatch journal directory: %v", err)
	}
	executionIdentity := strings.Repeat("a", 64)
	dispatchIdentity := strings.Repeat("b", 64)
	writeReleaseEvidenceFixtureJSON(t, filepath.Join(executionDir, executionIdentity+".json"), releasecmd.ReleaseExecutionJournal{
		SchemaVersion: 1,
		Identity: releasecmd.ReleaseExecutionIdentity{
			SHA256: executionIdentity,
		},
		UnitID:                  "api",
		NextVersion:             "2.3.5",
		Tag:                     "api/v2.3.5",
		State:                   releasecmd.ReleaseExecutionCommitCreated,
		PendingAction:           releasecmd.ReleaseExecutionPendingNone,
		ReleaseCommitSHA:        strings.Repeat("c", 40),
		DispatchJournalIdentity: dispatchIdentity,
		CreatedAt:               time.Date(2026, time.July, 26, 11, 0, 0, 0, time.UTC),
		UpdatedAt:               time.Date(2026, time.July, 26, 11, 1, 0, 0, time.UTC),
	})
	writeReleaseEvidenceFixtureJSON(t, filepath.Join(dispatchDir, dispatchIdentity+".json"), releasecmd.DispatchJournal{
		SchemaVersion: 1,
		Identity:      releasecmd.ReleaseDispatchIdentity{SHA256: dispatchIdentity},
		UnitID:        "api",
		Version:       "2.3.5",
		Tag:           "api/v2.3.5",
		State:         releasecmd.DispatchJournalRejected,
		LastError:     "Authorization: Bearer evidence-transport-secret",
		CreatedAt:     time.Date(2026, time.July, 26, 11, 2, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, time.July, 26, 11, 3, 0, 0, time.UTC),
	})
	if err := os.WriteFile(
		filepath.Join(dispatchDir, strings.Repeat("e", 64)+".json"),
		[]byte("{not-json"),
		0o600,
	); err != nil {
		t.Fatalf("write corrupt dispatch evidence: %v", err)
	}
	return root
}

type releaseEvidenceArchiveFixture struct {
	root          string
	identity      string
	digest        string
	sourcePath    string
	archivePath   string
	unrelatedPath string
	sourceBytes   []byte
}

func newReleaseEvidenceArchiveTransportRepository(t *testing.T) releaseEvidenceArchiveFixture {
	t.Helper()
	root := newReleaseLifecycleV2Repository(t)
	executionDir, err := releasecmd.NewReleaseExecutionJournalStore(root).JournalDirectory()
	if err != nil {
		t.Fatalf("execution journal directory: %v", err)
	}
	identity := strings.Repeat("a", 64)
	sourcePath := filepath.Join(executionDir, identity+".json")
	sourceBytes := writeReleaseEvidenceFixtureJSON(t, sourcePath, releasecmd.ReleaseExecutionJournal{
		SchemaVersion: 1,
		Identity:      releasecmd.ReleaseExecutionIdentity{SHA256: identity},
		UnitID:        "api",
		NextVersion:   "2.3.5",
		Tag:           "api/v2.3.5",
		State:         releasecmd.ReleaseExecutionHandoffReady,
		PendingAction: releasecmd.ReleaseExecutionPendingNone,
		CreatedAt:     time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, time.July, 26, 12, 1, 0, 0, time.UTC),
	})
	digest := releaseEvidenceSHA256(sourceBytes)
	unrelatedIdentity := strings.Repeat("f", 64)
	unrelatedPath := filepath.Join(executionDir, unrelatedIdentity+".json")
	writeReleaseEvidenceFixtureJSON(t, unrelatedPath, releasecmd.ReleaseExecutionJournal{
		SchemaVersion: 1,
		Identity:      releasecmd.ReleaseExecutionIdentity{SHA256: unrelatedIdentity},
		UnitID:        "worker",
		NextVersion:   "1.0.1",
		Tag:           "worker/v1.0.1",
		State:         releasecmd.ReleaseExecutionPrepared,
		PendingAction: releasecmd.ReleaseExecutionPendingNone,
		CreatedAt:     time.Date(2026, time.July, 26, 12, 2, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, time.July, 26, 12, 3, 0, 0, time.UTC),
	})
	archivePath := filepath.Join(executionDir, "archived", identity+"-"+digest+".json")
	return releaseEvidenceArchiveFixture{
		root: root, identity: identity, digest: digest,
		sourcePath: sourcePath, archivePath: archivePath,
		unrelatedPath: unrelatedPath, sourceBytes: sourceBytes,
	}
}

func writeReleaseEvidenceFixtureJSON(t *testing.T, path string, value any) []byte {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create evidence directory: %v", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal evidence fixture: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write evidence fixture: %v", err)
	}
	return data
}

func releaseEvidenceSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func releaseEvidenceModeName(mode releaseReadonlyMode) string {
	switch {
	case mode.format == "json":
		return "json"
	case mode.describe && mode.verbose:
		return "describe-verbose"
	case mode.describe:
		return "describe"
	case mode.verbose:
		return "verbose"
	default:
		return "default"
	}
}
