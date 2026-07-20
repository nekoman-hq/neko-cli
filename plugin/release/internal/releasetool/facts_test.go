package releasetool

import (
	"reflect"
	"testing"
)

func TestIdentityAndConfigurationFacts(t *testing.T) {
	tests := []struct {
		identity   Identity
		candidates []string
	}{
		{identity: GoReleaser, candidates: []string{".goreleaser.yml", ".goreleaser.yaml"}},
		{identity: JReleaser, candidates: []string{"jreleaser.yml"}},
		{identity: ReleaseIt, candidates: []string{".release-it.json"}},
	}
	if got := Identities(); !reflect.DeepEqual(got, []Identity{GoReleaser, JReleaser, ReleaseIt}) {
		t.Fatalf("identity order = %v", got)
	}
	for _, tt := range tests {
		t.Run(string(tt.identity), func(t *testing.T) {
			parsed, err := ParseIdentity(string(tt.identity))
			if err != nil || parsed != tt.identity {
				t.Fatalf("ParseIdentity(%q) = %q, %v", tt.identity, parsed, err)
			}
			first, err := ConfigCandidates(tt.identity)
			if err != nil {
				t.Fatalf("ConfigCandidates(%q): %v", tt.identity, err)
			}
			if !reflect.DeepEqual(first, tt.candidates) {
				t.Fatalf("candidates = %v, want %v", first, tt.candidates)
			}
			first[0] = "changed-by-caller"
			second, err := ConfigCandidates(tt.identity)
			if err != nil {
				t.Fatalf("ConfigCandidates(%q) again: %v", tt.identity, err)
			}
			if !reflect.DeepEqual(second, tt.candidates) {
				t.Fatalf("caller mutation leaked into candidates: %v", second)
			}
		})
	}
}

func TestIdentityFactsRejectUnknownValues(t *testing.T) {
	if _, err := ParseIdentity("GORELEASER"); err == nil {
		t.Fatal("case-insensitive identity was accepted")
	}
	if _, err := ConfigCandidates(Identity("semantic-release")); err == nil {
		t.Fatal("unknown config candidates were accepted")
	}
	if _, err := V1BehaviorFor(Identity("semantic-release")); err == nil {
		t.Fatal("unknown V1 behavior was accepted")
	}
}

func TestV1BehaviorFactsPreserveToolOwnership(t *testing.T) {
	tests := []struct {
		identity           Identity
		commitOwner        string
		tagOwner           string
		updatesVersionFile bool
		supportsDryRun     bool
		stateGuaranteed    bool
	}{
		{GoReleaser, "neko-cli", "neko-cli", false, true, true},
		{JReleaser, "neko-cli", "jreleaser", true, true, true},
		{ReleaseIt, "release-it", "release-it", true, false, false},
	}
	for _, tt := range tests {
		behavior, err := V1BehaviorFor(tt.identity)
		if err != nil {
			t.Fatalf("V1BehaviorFor(%q): %v", tt.identity, err)
		}
		if behavior.Identity != tt.identity || behavior.CommitOwner != tt.commitOwner || behavior.TagOwner != tt.tagOwner ||
			behavior.UpdatesVersionFiles != tt.updatesVersionFile || behavior.SupportsDryRun != tt.supportsDryRun ||
			behavior.StateCommitGuaranteed != tt.stateGuaranteed {
			t.Errorf("unexpected behavior for %s: %#v", tt.identity, behavior)
		}
		if !behavior.CreatesCommit || !behavior.CreatesTag || !behavior.Pushes || !behavior.CreatesGitHubRelease ||
			!behavior.SupportsLocalExecution || !behavior.RequiresRepositoryCleanliness || !behavior.MayRequireRollback ||
			!behavior.StateBeforeExecutor {
			t.Errorf("incomplete V1 ownership facts for %s: %#v", tt.identity, behavior)
		}
	}
}
