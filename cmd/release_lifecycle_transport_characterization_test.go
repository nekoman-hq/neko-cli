package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReleaseLifecycleDryRunsUseSharedPresentationAcrossCorePluginTransport(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositories := []struct {
		name  string
		root  string
		flags []string
	}{
		{name: "V1", root: newReleaseLifecycleV1Repository(t), flags: []string{"--unit", "default", "--dry-run"}},
		{name: "V2", root: newReleaseLifecycleV2Repository(t), flags: []string{"--unit", "api", "--dry-run"}},
	}

	for _, repository := range repositories {
		for _, command := range []string{"patch", "minor", "major"} {
			t.Run(repository.name+"/"+command, func(t *testing.T) {
				defaultOutput, defaultErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{},
				)
				describeOutput, describeErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{describe: true},
				)
				verboseOutput, verboseErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{verbose: true},
				)
				combinedOutput, combinedErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{describe: true, verbose: true},
				)
				if defaultErr != nil || describeErr != nil || verboseErr != nil || combinedErr != nil {
					t.Fatalf(
						"lifecycle transport exits: default=%v describe=%v verbose=%v combined=%v\ndefault output:\n%s\ndescribe output:\n%s\nverbose output:\n%s\ncombined output:\n%s",
						defaultErr, describeErr, verboseErr, combinedErr,
						defaultOutput, describeOutput, verboseOutput, combinedOutput,
					)
				}

				for _, want := range []string{
					"Release Summary",
					"Requested change",
					strings.ToUpper(command[:1]) + command[1:],
					"Dry run",
					"Operations",
					"Materialized Files",
					"Mutation boundary",
				} {
					if !strings.Contains(defaultOutput, want) {
						t.Fatalf("%s %s default omitted %q:\n%s", repository.name, command, want, defaultOutput)
					}
				}
				for _, hidden := range []string{
					"Source and Configuration",
					"Execution Evidence",
					"Git and Handoff",
				} {
					if strings.Contains(defaultOutput, hidden) {
						t.Fatalf("%s %s default exposed describe-only %q:\n%s", repository.name, command, hidden, defaultOutput)
					}
					if !strings.Contains(describeOutput, hidden) {
						t.Fatalf("%s %s describe omitted %q:\n%s", repository.name, command, hidden, describeOutput)
					}
				}
				if verboseOutput == defaultOutput || combinedOutput == describeOutput {
					t.Fatalf("%s %s verbose did not add execution phases", repository.name, command)
				}
				for _, output := range []string{defaultOutput, describeOutput, verboseOutput, combinedOutput} {
					if strings.Contains(output, repository.root) || strings.Contains(output, "\x1b[") {
						t.Fatalf("%s %s output exposed fixture root or ANSI:\n%s", repository.name, command, output)
					}
				}
			})
		}
	}
}

func TestReleaseLifecycleJSONAndExitBehaviorAreInvariantAcrossGlobalModes(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositories := []struct {
		name  string
		root  string
		flags []string
	}{
		{name: "V1", root: newReleaseLifecycleV1Repository(t), flags: []string{"--unit", "default", "--dry-run"}},
		{name: "V2", root: newReleaseLifecycleV2Repository(t), flags: []string{"--unit", "api", "--dry-run"}},
	}

	for _, repository := range repositories {
		for _, command := range []string{"patch", "minor", "major"} {
			t.Run(repository.name+"/"+command, func(t *testing.T) {
				plain, plainErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{format: "json"},
				)
				described, describedErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{format: "json", describe: true},
				)
				verboseOutput, verboseErr := executeReleaseReadonlyCommand(
					t, manifest, repository.root, command, repository.flags, releaseReadonlyMode{format: "json", verbose: true},
				)
				if !samePluginExit(plainErr, describedErr) || !samePluginExit(plainErr, verboseErr) {
					t.Fatalf("global presentation flags changed %s exit behavior: plain=%v describe=%v verbose=%v", command, plainErr, describedErr, verboseErr)
				}
				plainResponse := decodeReleaseReadonlyPublicResponse(t, plain)
				if !reflect.DeepEqual(plainResponse.Data, decodeReleaseReadonlyPublicResponse(t, described).Data) ||
					!reflect.DeepEqual(plainResponse.Data, decodeReleaseReadonlyPublicResponse(t, verboseOutput).Data) {
					t.Fatalf("%s global presentation flags changed domain JSON", command)
				}
				for _, output := range []string{plain, described, verboseOutput} {
					for _, forbidden := range []string{
						"human_table", "human_properties", "describe_only", "\x1b[", "lifecycle-transport-secret",
					} {
						if strings.Contains(output, forbidden) {
							t.Fatalf("%s JSON contains %q:\n%s", command, forbidden, output)
						}
					}
				}
			})
		}
	}
}

func TestReleaseResumeTransportDistinguishesRefusalAndResumableDryRun(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	t.Setenv("GITHUB_TOKEN", "lifecycle-transport-secret")
	t.Setenv("GH_TOKEN", "lifecycle-transport-secret")

	noJournalRoot := newReleaseLifecycleV2Repository(t)
	refusal, refusalErr := executeReleaseReadonlyCommand(
		t,
		manifest,
		noJournalRoot,
		"resume",
		[]string{"--unit", "api", "--dry-run"},
		releaseReadonlyMode{},
	)
	if refusalErr == nil || !isResponseProcessExitError(refusalErr) || ProcessExitCode(refusalErr) != 1 {
		t.Fatalf("resume no-journal exit = %v, want explicit response exit 1", refusalErr)
	}
	for _, want := range []string{"Resume Refused", "NO_RESUMABLE_JOURNAL", "no resumable V2 release execution journal found for unit api"} {
		if !strings.Contains(refusal, want) {
			t.Fatalf("resume no-journal default omitted %q:\n%s", want, refusal)
		}
	}

	resumableRoot := newReleaseLifecycleResumeRepository(t)
	defaultOutput, defaultErr := executeReleaseReadonlyCommand(
		t,
		manifest,
		resumableRoot,
		"resume",
		[]string{"--unit", "api", "--dry-run"},
		releaseReadonlyMode{},
	)
	describeOutput, describeErr := executeReleaseReadonlyCommand(
		t,
		manifest,
		resumableRoot,
		"resume",
		[]string{"--unit", "api", "--dry-run"},
		releaseReadonlyMode{describe: true},
	)
	verboseOutput, verboseErr := executeReleaseReadonlyCommand(
		t,
		manifest,
		resumableRoot,
		"resume",
		[]string{"--unit", "api", "--dry-run"},
		releaseReadonlyMode{verbose: true},
	)
	if defaultErr != nil || describeErr != nil || verboseErr != nil {
		t.Fatalf("resume dry-run transport exits: default=%v describe=%v verbose=%v", defaultErr, describeErr, verboseErr)
	}
	for _, want := range []string{
		"Resume Summary", "Journal phase", "Pending action", "Resume eligibility",
		"Retry safety", "Planned Continuation", "Mutation boundary", "Dry run",
	} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("resume dry-run default omitted %q:\n%s", want, defaultOutput)
		}
	}
	for _, want := range []string{
		"Recovery Journal", "Local Git Evidence", "Recovery Assessment", "Continuation and Handoff", "Limitations",
	} {
		if !strings.Contains(describeOutput, want) {
			t.Fatalf("resume describe omitted %q:\n%s", want, describeOutput)
		}
	}
	if verboseOutput == defaultOutput {
		t.Fatalf("resume verbose did not add orchestration phases")
	}
	for _, output := range []string{refusal, defaultOutput, describeOutput, verboseOutput} {
		if strings.Contains(output, resumableRoot) || strings.Contains(output, noJournalRoot) ||
			strings.Contains(output, "lifecycle-transport-secret") || strings.Contains(output, "\x1b[") {
			t.Fatalf("resume output exposed a path, credential, or ANSI:\n%s", output)
		}
	}
}

func TestReleaseLifecycleRedirectedNoColorOutputIsANSIAndCredentialFree(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	root := newReleaseLifecycleV2Repository(t)
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GITHUB_TOKEN", "lifecycle-transport-secret")
	t.Setenv("GH_TOKEN", "lifecycle-transport-secret")

	for _, command := range []string{"patch", "minor", "major"} {
		for _, mode := range []releaseReadonlyMode{{}, {describe: true}, {verbose: true}, {describe: true, verbose: true}} {
			output, err := executeReleaseReadonlyCommand(
				t, manifest, root, command, []string{"--unit", "api", "--dry-run"}, mode,
			)
			if err != nil {
				t.Fatalf("%s redirected mode %#v: %v", command, mode, err)
			}
			if strings.Contains(output, "\x1b[") || strings.Contains(output, "lifecycle-transport-secret") {
				t.Fatalf("%s redirected output is unsafe in mode %#v:\n%s", command, mode, output)
			}
		}
	}
}

func TestReleaseAndResumeDryRunPresentationModesDoNotMutateFixtures(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositories := []struct {
		name     string
		root     string
		commands []string
		flags    []string
	}{
		{
			name: "V1", root: newReleaseLifecycleV1Repository(t),
			commands: []string{"patch", "minor", "major"},
			flags:    []string{"--unit", "default", "--dry-run"},
		},
		{
			name: "V2", root: newReleaseLifecycleV2Repository(t),
			commands: []string{"patch", "minor", "major"},
			flags:    []string{"--unit", "api", "--dry-run"},
		},
		{
			name: "Resume", root: newReleaseLifecycleResumeRepository(t),
			commands: []string{"resume"},
			flags:    []string{"--unit", "api", "--dry-run"},
		},
	}
	modes := []releaseReadonlyMode{
		{},
		{describe: true},
		{verbose: true},
		{describe: true, verbose: true},
		{format: "json"},
		{format: "json", describe: true},
		{format: "json", verbose: true},
	}

	for _, repository := range repositories {
		t.Run(repository.name, func(t *testing.T) {
			beforeHead := strings.TrimSpace(runReleaseReadonlyGit(t, repository.root, "rev-parse", "HEAD"))
			beforeStatus := runReleaseReadonlyGit(t, repository.root, "status", "--porcelain", "--untracked-files=all")
			beforeTags := runReleaseReadonlyGit(t, repository.root, "tag", "--list")
			beforeV1, _ := os.ReadFile(filepath.Join(repository.root, ".release.neko.json"))
			beforeV2, _ := os.ReadFile(filepath.Join(repository.root, ".neko", "release.state.json"))
			beforeJournals := releaseLifecycleJournalSnapshot(t, repository.root)

			for _, command := range repository.commands {
				for _, mode := range modes {
					if _, err := executeReleaseReadonlyCommand(
						t, manifest, repository.root, command, repository.flags, mode,
					); err != nil {
						t.Fatalf("%s %s mode %#v: %v", repository.name, command, mode, err)
					}
				}
			}

			if after := strings.TrimSpace(runReleaseReadonlyGit(t, repository.root, "rev-parse", "HEAD")); after != beforeHead {
				t.Fatalf("%s dry-run changed HEAD: before=%s after=%s", repository.name, beforeHead, after)
			}
			if after := runReleaseReadonlyGit(t, repository.root, "status", "--porcelain", "--untracked-files=all"); after != beforeStatus {
				t.Fatalf("%s dry-run changed worktree/index: before=%q after=%q", repository.name, beforeStatus, after)
			}
			if after := runReleaseReadonlyGit(t, repository.root, "tag", "--list"); after != beforeTags {
				t.Fatalf("%s dry-run changed tags: before=%q after=%q", repository.name, beforeTags, after)
			}
			afterV1, _ := os.ReadFile(filepath.Join(repository.root, ".release.neko.json"))
			afterV2, _ := os.ReadFile(filepath.Join(repository.root, ".neko", "release.state.json"))
			if string(afterV1) != string(beforeV1) || string(afterV2) != string(beforeV2) {
				t.Fatalf("%s dry-run rewrote release state/config", repository.name)
			}
			if after := releaseLifecycleJournalSnapshot(t, repository.root); !reflect.DeepEqual(after, beforeJournals) {
				t.Fatalf("%s dry-run changed journal files: before=%#v after=%#v", repository.name, beforeJournals, after)
			}
		})
	}
}

func TestReleaseResumeJSONIsInvariantAcrossGlobalPresentationModes(t *testing.T) {
	manifest := installReleaseReadonlyHelperPlugin(t)
	repositories := []struct {
		name string
		root string
	}{
		{name: "no journal refusal", root: newReleaseLifecycleV2Repository(t)},
		{name: "resumable assessment", root: newReleaseLifecycleResumeRepository(t)},
	}
	flags := []string{"--unit", "api", "--dry-run"}

	for _, repository := range repositories {
		t.Run(repository.name, func(t *testing.T) {
			plain, plainErr := executeReleaseReadonlyCommand(
				t, manifest, repository.root, "resume", flags, releaseReadonlyMode{format: "json"},
			)
			described, describedErr := executeReleaseReadonlyCommand(
				t, manifest, repository.root, "resume", flags, releaseReadonlyMode{format: "json", describe: true},
			)
			verboseOutput, verboseErr := executeReleaseReadonlyCommand(
				t, manifest, repository.root, "resume", flags, releaseReadonlyMode{format: "json", verbose: true},
			)
			if !samePluginExit(plainErr, describedErr) || !samePluginExit(plainErr, verboseErr) {
				t.Fatalf("resume global modes changed exit behavior: plain=%v describe=%v verbose=%v", plainErr, describedErr, verboseErr)
			}
			plainResponse := decodeReleaseReadonlyPublicResponse(t, plain)
			describeResponse := decodeReleaseReadonlyPublicResponse(t, described)
			verboseResponse := decodeReleaseReadonlyPublicResponse(t, verboseOutput)
			if plainResponse.Status != describeResponse.Status || plainResponse.Status != verboseResponse.Status ||
				!reflect.DeepEqual(plainResponse.Data, describeResponse.Data) ||
				!reflect.DeepEqual(plainResponse.Data, verboseResponse.Data) {
				t.Fatalf("resume presentation flags changed response status or domain data")
			}
			for _, output := range []string{plain, described, verboseOutput} {
				for _, forbidden := range []string{"human_table", "human_properties", "describe_only", "\x1b["} {
					if strings.Contains(output, forbidden) {
						t.Fatalf("resume JSON contains %q:\n%s", forbidden, output)
					}
				}
			}
		})
	}
}

func releaseLifecycleJournalSnapshot(t *testing.T, root string) []string {
	t.Helper()
	commonDir := strings.TrimSpace(runReleaseReadonlyGit(t, root, "rev-parse", "--git-common-dir"))
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(root, commonDir)
	}
	journalRoot := filepath.Join(commonDir, "neko", "release")
	var paths []string
	err := filepath.WalkDir(journalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, relativeErr := filepath.Rel(journalRoot, path)
		if relativeErr != nil {
			return relativeErr
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		paths = append(paths, filepath.ToSlash(relative)+"\x00"+string(data))
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("snapshot release journals: %v", err)
	}
	return paths
}
