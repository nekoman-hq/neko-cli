package release

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

var fullReleaseObjectIDPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// GitObjectFormat identifies the repository object-ID format validated for a
// dispatched release context.
type GitObjectFormat string

const (
	GitObjectFormatSHA1   GitObjectFormat = "sha1"
	GitObjectFormatSHA256 GitObjectFormat = "sha256"
)

// ReleaseContextValidationRequest is the typed application input for checking
// one dispatched Release V2 context against an explicit repository root.
//
//nolint:govet // Fields follow the public command contract order.
type ReleaseContextValidationRequest struct {
	RepositoryRoot string
	UnitID         string
	Version        string
	Tag            string
	ReleaseSHA     string
}

// ValidatedReleaseContext contains only canonical local release facts that
// have passed V2 source, unit, version, tag, commit, HEAD, and tag-target checks.
//
//nolint:govet // Fields follow stable output order.
type ValidatedReleaseContext struct {
	UnitID           string
	DisplayName      string
	Version          string
	TagPrefix        string
	Tag              string
	ReleaseSHA       string
	WorkingDirectory string
	Executor         string
	Delivery         string
	Workflow         string
	GitObjectFormat  GitObjectFormat
	HeadMatches      bool
	TagTargetMatches bool
}

type releaseContextSourceReader interface {
	ReadV2(string) (*releaseconfig.ReleaseRepository, *CommandFailure)
}

type releaseContextGitReader interface {
	ObjectFormat(string) (GitObjectFormat, error)
	ObjectType(string, string) (string, error)
	HeadCommit(string) (string, error)
	TagExists(string, string) (bool, error)
	TagCommit(string, string) (string, error)
}

type releaseContextValidationUseCase struct {
	sources releaseContextSourceReader
	git     releaseContextGitReader
}

func newReleaseContextValidationUseCase() releaseContextValidationUseCase {
	return releaseContextValidationUseCase{
		sources: filesystemReleaseContextSourceReader{},
		git:     releaseContextGitAdapter{runner: execGitRunner{}},
	}
}

func (useCase releaseContextValidationUseCase) Validate(_ context.Context, request ReleaseContextValidationRequest) (*ValidatedReleaseContext, *CommandFailure) {
	if failure := validateReleaseContextRequestSyntax(request); failure != nil {
		return nil, failure
	}
	repository, failure := useCase.sources.ReadV2(request.RepositoryRoot)
	if failure != nil {
		return nil, failure
	}
	unit, err := releaseconfig.ResolveReleaseUnit(repository, request.UnitID, releaseconfig.UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		return nil, failureFromMessage("RELEASE_UNIT_NOT_FOUND", "the dispatched release unit is not present exactly once in V2 config and state")
	}
	if failure := validateReleaseContextVersion(*unit, request.Version); failure != nil {
		return nil, failure
	}
	if failure := validateReleaseContextTag(*unit, request.Version, request.Tag); failure != nil {
		return nil, failure
	}

	objectFormat, err := useCase.git.ObjectFormat(request.RepositoryRoot)
	if err != nil {
		return nil, failureFromMessage("GIT_REPOSITORY_UNAVAILABLE", "the repository object format could not be read from local Git data")
	}
	if failure := validateReleaseObjectIDForFormat(request.ReleaseSHA, objectFormat); failure != nil {
		return nil, failure
	}
	objectType, err := useCase.git.ObjectType(request.RepositoryRoot, request.ReleaseSHA)
	if err != nil || objectType != "commit" {
		return nil, failureFromMessage("RELEASE_SHA_NOT_COMMIT", "release_sha must identify an existing local commit object")
	}
	head, err := useCase.git.HeadCommit(request.RepositoryRoot)
	if err != nil {
		return nil, failureFromMessage("HEAD_UNAVAILABLE", "checked-out HEAD could not be resolved to a local commit")
	}
	if head != request.ReleaseSHA {
		return nil, failureFromMessage("HEAD_MISMATCH", "checked-out HEAD does not match release_sha")
	}
	tagExists, err := useCase.git.TagExists(request.RepositoryRoot, request.Tag)
	if err != nil {
		return nil, failureFromMessage("TAG_HISTORY_UNAVAILABLE", "local tag history could not be inspected; ensure the checkout contains complete tag history")
	}
	if !tagExists {
		return nil, failureFromMessage("RELEASE_TAG_MISSING", "the dispatched release tag is missing locally; fetch complete tag history before validation")
	}
	tagCommit, err := useCase.git.TagCommit(request.RepositoryRoot, request.Tag)
	if err != nil {
		return nil, failureFromMessage("TAG_TARGET_INVALID", "the dispatched release tag does not resolve to a local commit")
	}
	if tagCommit != request.ReleaseSHA {
		return nil, failureFromMessage("TAG_TARGET_MISMATCH", "the dispatched release tag does not resolve to release_sha")
	}

	return &ValidatedReleaseContext{
		UnitID:           unit.ID,
		DisplayName:      unit.DisplayName,
		Version:          request.Version,
		TagPrefix:        unit.TagPrefix,
		Tag:              request.Tag,
		ReleaseSHA:       request.ReleaseSHA,
		WorkingDirectory: unit.WorkingDirectory,
		Executor:         unit.ExecutorType,
		Delivery:         unit.Delivery,
		Workflow:         unit.Workflow,
		GitObjectFormat:  objectFormat,
		HeadMatches:      true,
		TagTargetMatches: true,
	}, nil
}

func validateReleaseContextRequestSyntax(request ReleaseContextValidationRequest) *CommandFailure {
	if !exactRequiredValue(request.RepositoryRoot) {
		return failureFromMessage("INVALID_CONTEXT_INPUT", "repository root is required for release context validation")
	}
	values := []struct {
		name  string
		value string
	}{
		{name: "unit", value: request.UnitID},
		{name: "version", value: request.Version},
		{name: "tag", value: request.Tag},
		{name: "release_sha", value: request.ReleaseSHA},
	}
	for _, value := range values {
		if !exactRequiredValue(value.value) {
			return failureFromMessage("INVALID_CONTEXT_INPUT", value.name+" is required and must not contain surrounding whitespace or control characters")
		}
	}
	if err := releaseconfig.ValidateReleaseUnitID(request.UnitID); err != nil {
		return failureFromMessage("INVALID_CONTEXT_INPUT", "unit is not a valid V2 release unit id")
	}
	canonicalVersion, err := releaseconfig.CanonicalReleaseVersion(request.Version)
	if err != nil || canonicalVersion != request.Version {
		return failureFromMessage("INVALID_CONTEXT_INPUT", "version must be canonical semantic version syntax")
	}
	if !fullReleaseObjectIDPattern.MatchString(request.ReleaseSHA) {
		return failureFromMessage("INVALID_RELEASE_SHA", "release_sha must be a full lowercase Git object ID, not an abbreviation")
	}
	return nil
}

func validateReleaseContextVersion(unit releaseconfig.ReleaseUnit, dispatchedVersion string) *CommandFailure {
	canonicalVersion, err := releaseconfig.CanonicalReleaseVersion(unit.Version)
	if err != nil {
		return failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "the authoritative V2 state version is invalid")
	}
	if canonicalVersion != dispatchedVersion {
		return failureFromMessage("RELEASE_VERSION_MISMATCH", "the dispatched version does not match the authoritative current V2 state version")
	}
	return nil
}

func validateReleaseContextTag(unit releaseconfig.ReleaseUnit, version, dispatchedTag string) *CommandFailure {
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "the selected release unit has an invalid tag policy")
	}
	if tagSpec.Format(version) != dispatchedTag {
		return failureFromMessage("RELEASE_TAG_MISMATCH", "the dispatched tag does not match the selected release unit and version")
	}
	return nil
}

func validateReleaseObjectIDForFormat(objectID string, objectFormat GitObjectFormat) *CommandFailure {
	expectedLength := 0
	switch objectFormat {
	case GitObjectFormatSHA1:
		expectedLength = 40
	case GitObjectFormatSHA256:
		expectedLength = 64
	default:
		return failureFromMessage("GIT_OBJECT_FORMAT_UNSUPPORTED", "the local Git repository uses an unsupported object format")
	}
	if len(objectID) != expectedLength {
		return failureFromMessage("INVALID_RELEASE_SHA", "release_sha is not a full object ID for the local Git repository format")
	}
	return nil
}

func exactRequiredValue(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) {
			return false
		}
	}
	return true
}
