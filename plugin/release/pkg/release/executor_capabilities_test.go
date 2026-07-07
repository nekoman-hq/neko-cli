package release

import "testing"

func TestResolveExecutorCapabilities(t *testing.T) {
	tests := []struct {
		executor           string
		commitOwner        string
		tagOwner           string
		updatesVersionFile bool
		supportsDryRun     bool
		stateGuaranteed    bool
	}{
		{
			executor:           "goreleaser",
			commitOwner:        "neko-cli",
			tagOwner:           "neko-cli",
			updatesVersionFile: false,
			supportsDryRun:     true,
			stateGuaranteed:    true,
		},
		{
			executor:           "jreleaser",
			commitOwner:        "neko-cli",
			tagOwner:           "jreleaser",
			updatesVersionFile: true,
			supportsDryRun:     true,
			stateGuaranteed:    true,
		},
		{
			executor:           "release-it",
			commitOwner:        "release-it",
			tagOwner:           "release-it",
			updatesVersionFile: true,
			supportsDryRun:     false,
			stateGuaranteed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.executor, func(t *testing.T) {
			capabilities, err := ResolveExecutorCapabilities(tt.executor)
			if err != nil {
				t.Fatalf("ResolveExecutorCapabilities(%q): %v", tt.executor, err)
			}
			if capabilities.Type != tt.executor {
				t.Fatalf("expected type %q, got %#v", tt.executor, capabilities)
			}
			if capabilities.UpdatesVersionFiles != tt.updatesVersionFile {
				t.Fatalf("unexpected UpdatesVersionFiles for %s: %#v", tt.executor, capabilities)
			}
			if capabilities.SupportsDryRun != tt.supportsDryRun {
				t.Fatalf("unexpected SupportsDryRun for %s: %#v", tt.executor, capabilities)
			}
			if !capabilities.CreatesCommit || !capabilities.CreatesTag || !capabilities.Pushes || !capabilities.CreatesGitHubRelease {
				t.Fatalf("expected release ownership capabilities for %s, got %#v", tt.executor, capabilities)
			}
			if !capabilities.SupportsLocalExecution || !capabilities.RequiresRepositoryCleanliness || !capabilities.MayRequireRollback {
				t.Fatalf("expected local execution guard capabilities for %s, got %#v", tt.executor, capabilities)
			}
			if capabilities.CommitOwner != tt.commitOwner || capabilities.TagOwner != tt.tagOwner {
				t.Fatalf("unexpected ownership for %s, got %#v", tt.executor, capabilities)
			}
			if capabilities.StateCommitGuaranteed != tt.stateGuaranteed {
				t.Fatalf("unexpected state guarantee for %s, got %#v", tt.executor, capabilities)
			}
			if !capabilities.StateBeforeExecutor {
				t.Fatalf("expected state-before-executor for %s, got %#v", tt.executor, capabilities)
			}
			if !tt.stateGuaranteed && capabilities.V2LocalExecutionBlockedReason == "" {
				t.Fatalf("expected blocked reason for unguaranteed executor %s", tt.executor)
			}
		})
	}
}

func TestResolveExecutorCapabilitiesRejectsUnknown(t *testing.T) {
	if _, err := ResolveExecutorCapabilities("semantic-release"); err == nil {
		t.Fatal("expected unknown executor error")
	}
}
