package releasetool

import "fmt"

// Identity is the canonical persisted and configured release-tool identity.
type Identity string

const (
	GoReleaser Identity = "goreleaser"
	JReleaser  Identity = "jreleaser"
	ReleaseIt  Identity = "release-it"
)

const (
	GoReleaserConfigFileYML  = ".goreleaser.yml"
	GoReleaserConfigFileYAML = ".goreleaser.yaml"
	JReleaserConfigFile      = "jreleaser.yml"
	ReleaseItConfigFile      = ".release-it.json"
)

// ParseIdentity validates one exact release-tool identity. Identity parsing is
// intentionally strict because the values are persisted wire facts.
func ParseIdentity(value string) (Identity, error) {
	identity := Identity(value)
	if !identity.Valid() {
		return "", fmt.Errorf("unknown release tool: %s", value)
	}
	return identity, nil
}

// Valid reports whether the identity names one supported release tool.
func (identity Identity) Valid() bool {
	switch identity {
	case GoReleaser, JReleaser, ReleaseIt:
		return true
	default:
		return false
	}
}

// Identities returns the supported identities in the stable command-facing
// order. A fresh slice prevents callers from mutating shared package state.
func Identities() []Identity {
	return []Identity{GoReleaser, JReleaser, ReleaseIt}
}

// ConfigCandidates returns the canonical ordered configuration filenames for
// the selected release tool. A fresh slice is returned on every call.
func ConfigCandidates(identity Identity) ([]string, error) {
	switch identity {
	case GoReleaser:
		return []string{GoReleaserConfigFileYML, GoReleaserConfigFileYAML}, nil
	case JReleaser:
		return []string{JReleaserConfigFile}, nil
	case ReleaseIt:
		return []string{ReleaseItConfigFile}, nil
	default:
		return nil, fmt.Errorf("unknown release tool: %s", identity)
	}
}

// V1Behavior records the static side-effect ownership of one retained V1
// execution adapter. It contains no executor instance or lifecycle state.
//
//nolint:govet // Logical capability order is clearer than field-alignment order.
type V1Behavior struct {
	Identity                      Identity
	UpdatesVersionFiles           bool
	CreatesCommit                 bool
	CreatesTag                    bool
	Pushes                        bool
	CreatesGitHubRelease          bool
	SupportsLocalExecution        bool
	SupportsDryRun                bool
	RequiresRepositoryCleanliness bool
	MayRequireRollback            bool
	VersionFilesOwner             string
	CommitOwner                   string
	TagOwner                      string
	PushOwner                     string
	GitHubReleaseOwner            string
	StateBeforeExecutor           bool
	StateCommitGuaranteed         bool
	StateCommitGuarantee          string
	V2LocalExecutionBlockedReason string
}

// V1BehaviorFor returns the characterized static V1 behavior for an identity.
func V1BehaviorFor(identity Identity) (V1Behavior, error) {
	switch identity {
	case GoReleaser:
		return V1Behavior{
			Identity:                      GoReleaser,
			UpdatesVersionFiles:           false,
			CreatesCommit:                 true,
			CreatesTag:                    true,
			Pushes:                        true,
			CreatesGitHubRelease:          true,
			SupportsLocalExecution:        true,
			SupportsDryRun:                true,
			RequiresRepositoryCleanliness: true,
			MayRequireRollback:            true,
			VersionFilesOwner:             "none",
			CommitOwner:                   "neko-cli",
			TagOwner:                      "neko-cli",
			PushOwner:                     "neko-cli",
			GitHubReleaseOwner:            string(GoReleaser),
			StateBeforeExecutor:           true,
			StateCommitGuaranteed:         true,
			StateCommitGuarantee:          "Neko CLI writes and stages .neko/release.state.json before its release commit",
		}, nil
	case JReleaser:
		return V1Behavior{
			Identity:                      JReleaser,
			UpdatesVersionFiles:           true,
			CreatesCommit:                 true,
			CreatesTag:                    true,
			Pushes:                        true,
			CreatesGitHubRelease:          true,
			SupportsLocalExecution:        true,
			SupportsDryRun:                true,
			RequiresRepositoryCleanliness: true,
			MayRequireRollback:            true,
			VersionFilesOwner:             "neko-cli",
			CommitOwner:                   "neko-cli",
			TagOwner:                      string(JReleaser),
			PushOwner:                     "neko-cli + jreleaser",
			GitHubReleaseOwner:            string(JReleaser),
			StateBeforeExecutor:           true,
			StateCommitGuaranteed:         true,
			StateCommitGuarantee:          "Neko CLI writes and stages .neko/release.state.json before its release metadata commit; JReleaser creates the tag and release later",
		}, nil
	case ReleaseIt:
		return V1Behavior{
			Identity:                      ReleaseIt,
			UpdatesVersionFiles:           true,
			CreatesCommit:                 true,
			CreatesTag:                    true,
			Pushes:                        true,
			CreatesGitHubRelease:          true,
			SupportsLocalExecution:        true,
			SupportsDryRun:                false,
			RequiresRepositoryCleanliness: true,
			MayRequireRollback:            true,
			VersionFilesOwner:             string(ReleaseIt),
			CommitOwner:                   string(ReleaseIt),
			TagOwner:                      string(ReleaseIt),
			PushOwner:                     string(ReleaseIt),
			GitHubReleaseOwner:            string(ReleaseIt),
			StateBeforeExecutor:           true,
			StateCommitGuaranteed:         false,
			StateCommitGuarantee:          "release-it owns commit, tag, push, and GitHub release creation; root V2 state cannot be guaranteed in that commit from a nested unit root",
			V2LocalExecutionBlockedReason: "V2 local release-it is blocked because .neko/release.state.json lives at the repository root and release-it owns the release commit from the unit root",
		}, nil
	default:
		return V1Behavior{}, fmt.Errorf("unknown release tool: %s", identity)
	}
}
