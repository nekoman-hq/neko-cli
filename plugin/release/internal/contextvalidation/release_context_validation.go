package contextvalidation

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releaseworkflow"
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
	Checks           []ReleaseContextCheck
}

// ReleaseContextCheck is presentation-only validation evidence. Machine data
// remains mapped explicitly from ValidatedReleaseContext's canonical fields.
type ReleaseContextCheck struct {
	Name     string
	Status   string
	Subject  string
	Expected string
	Actual   string
	Guidance string
}

type releaseContextSourceReader interface {
	ReadV2(string) (*releaseconfig.ReleaseRepository, *commandFailure)
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
		git:     releaseContextGitAdapter{runner: execContextGitRunner{}},
	}
}

//nolint:funlen // Validation order is the product contract; extracting phases would obscure fail precedence.
func (useCase releaseContextValidationUseCase) Validate(_ context.Context, request ReleaseContextValidationRequest) (*ValidatedReleaseContext, *commandFailure) {
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
	result := &ValidatedReleaseContext{
		UnitID: request.UnitID, DisplayName: unit.DisplayName, Version: request.Version,
		TagPrefix: unit.TagPrefix, Tag: request.Tag, ReleaseSHA: request.ReleaseSHA,
		WorkingDirectory: unit.WorkingDirectory, Executor: unit.ExecutorType,
		Delivery: unit.Delivery, Workflow: unit.Workflow,
		Checks: []ReleaseContextCheck{
			passedReleaseContextCheck("Release source", "V2 config and state"),
			passedReleaseContextCheck("Release unit", request.UnitID),
		},
	}
	versionFailure := validateReleaseContextVersion(*unit, request.Version)
	result.Checks = append(result.Checks, releaseContextCheck(
		"Version", unit.Version, request.Version, versionFailure,
	))
	tagSpec, tagSpecErr := releaseconfig.NewTagSpec(unit.TagPrefix)
	expectedTag := ""
	if tagSpecErr == nil {
		expectedTag = tagSpec.Format(request.Version)
	}
	tagFailure := validateReleaseContextTag(*unit, request.Version, request.Tag)
	result.Checks = append(result.Checks, releaseContextCheck(
		"Tag", expectedTag, request.Tag, tagFailure,
	))
	if versionFailure != nil {
		return releaseContextValidationFailure(result, versionFailure)
	}
	if tagFailure != nil {
		return releaseContextValidationFailure(result, tagFailure)
	}

	objectFormat, err := useCase.git.ObjectFormat(request.RepositoryRoot)
	if err != nil {
		failure := failureFromMessage("GIT_REPOSITORY_UNAVAILABLE", "the repository object format could not be read from local Git data")
		result.Checks = append(result.Checks, failedReleaseContextCheck("Git object format", "local repository", failure))
		return releaseContextValidationFailure(result, failure)
	}
	result.GitObjectFormat = objectFormat
	result.Checks = append(result.Checks, passedReleaseContextCheck("Git object format", string(objectFormat)))
	if failure := validateReleaseObjectIDForFormat(request.ReleaseSHA, objectFormat); failure != nil {
		result.Checks = append(result.Checks, failedReleaseContextCheck("Release commit format", request.ReleaseSHA, failure))
		return releaseContextValidationFailure(result, failure)
	}
	result.Checks = append(result.Checks, passedReleaseContextCheck("Release commit format", request.ReleaseSHA))
	objectType, err := useCase.git.ObjectType(request.RepositoryRoot, request.ReleaseSHA)
	if err != nil || objectType != "commit" {
		failure := failureFromMessage("RELEASE_SHA_NOT_COMMIT", "release_sha must identify an existing local commit object")
		result.Checks = append(result.Checks, failedReleaseContextCheck("Release commit object", request.ReleaseSHA, failure))
		return releaseContextValidationFailure(result, failure)
	}
	result.Checks = append(result.Checks, passedReleaseContextCheck("Release commit object", request.ReleaseSHA))
	head, err := useCase.git.HeadCommit(request.RepositoryRoot)
	if err != nil {
		failure := failureFromMessage("HEAD_UNAVAILABLE", "checked-out HEAD could not be resolved to a local commit")
		result.Checks = append(result.Checks, failedReleaseContextCheck("Checked-out HEAD", request.ReleaseSHA, failure))
		return releaseContextValidationFailure(result, failure)
	}
	if head != request.ReleaseSHA {
		failure := failureFromMessage("HEAD_MISMATCH", "checked-out HEAD does not match release_sha")
		result.Checks = append(result.Checks, releaseContextCheck("Checked-out HEAD", request.ReleaseSHA, head, failure))
		return releaseContextValidationFailure(result, failure)
	}
	result.HeadMatches = true
	result.Checks = append(result.Checks, passedReleaseContextCheck("Checked-out HEAD", head))
	tagExists, err := useCase.git.TagExists(request.RepositoryRoot, request.Tag)
	if err != nil {
		failure := failureFromMessage("TAG_HISTORY_UNAVAILABLE", "local tag history could not be inspected; ensure the checkout contains complete tag history")
		result.Checks = append(result.Checks, failedReleaseContextCheck("Release tag history", request.Tag, failure))
		return releaseContextValidationFailure(result, failure)
	}
	if !tagExists {
		failure := failureFromMessage("RELEASE_TAG_MISSING", "the dispatched release tag is missing locally; fetch complete tag history before validation")
		result.Checks = append(result.Checks, failedReleaseContextCheck("Release tag history", request.Tag, failure))
		return releaseContextValidationFailure(result, failure)
	}
	result.Checks = append(result.Checks, passedReleaseContextCheck("Release tag history", request.Tag))
	tagCommit, err := useCase.git.TagCommit(request.RepositoryRoot, request.Tag)
	if err != nil {
		failure := failureFromMessage("TAG_TARGET_INVALID", "the dispatched release tag does not resolve to a local commit")
		result.Checks = append(result.Checks, failedReleaseContextCheck("Release tag target", request.Tag, failure))
		return releaseContextValidationFailure(result, failure)
	}
	if tagCommit != request.ReleaseSHA {
		failure := failureFromMessage("TAG_TARGET_MISMATCH", "the dispatched release tag does not resolve to release_sha")
		result.Checks = append(result.Checks, releaseContextCheck("Release tag target", request.ReleaseSHA, tagCommit, failure))
		return releaseContextValidationFailure(result, failure)
	}
	result.TagTargetMatches = true
	result.Checks = append(result.Checks, passedReleaseContextCheck("Release tag target", tagCommit))
	return result, nil
}

func releaseContextValidationFailure(
	result *ValidatedReleaseContext,
	failure *commandFailure,
) (*ValidatedReleaseContext, *commandFailure) {
	failure.Context = result
	return nil, failure
}

func passedReleaseContextCheck(name, subject string) ReleaseContextCheck {
	return ReleaseContextCheck{Name: name, Status: "passed", Subject: subject}
}

func failedReleaseContextCheck(name, subject string, failure *commandFailure) ReleaseContextCheck {
	return ReleaseContextCheck{
		Name: name, Status: "failed", Subject: subject,
		Guidance: releaseContextFailureGuidance(failure),
	}
}

func releaseContextCheck(name, expected, actual string, failure *commandFailure) ReleaseContextCheck {
	if failure == nil {
		return ReleaseContextCheck{Name: name, Status: "passed", Subject: actual, Expected: expected, Actual: actual}
	}
	return ReleaseContextCheck{
		Name: name, Status: "failed", Subject: actual, Expected: expected, Actual: actual,
		Guidance: releaseContextFailureGuidance(failure),
	}
}

func releaseContextFailureGuidance(failure *commandFailure) string {
	if failure == nil {
		return ""
	}
	return failure.Message
}

func validateReleaseContextRequestSyntax(request ReleaseContextValidationRequest) *commandFailure {
	if !exactRequiredValue(request.RepositoryRoot) {
		return failureFromMessage("INVALID_CONTEXT_INPUT", "repository root is required for release context validation")
	}
	values := []struct {
		name  string
		value string
	}{
		{name: releaseworkflow.DispatchInputUnit, value: request.UnitID},
		{name: releaseworkflow.DispatchInputVersion, value: request.Version},
		{name: releaseworkflow.DispatchInputTag, value: request.Tag},
		{name: releaseworkflow.DispatchInputReleaseSHA, value: request.ReleaseSHA},
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

func validateReleaseContextVersion(unit releaseconfig.ReleaseUnit, dispatchedVersion string) *commandFailure {
	canonicalVersion, err := releaseconfig.CanonicalReleaseVersion(unit.Version)
	if err != nil {
		return failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "the authoritative V2 state version is invalid")
	}
	if canonicalVersion != dispatchedVersion {
		return failureFromMessage("RELEASE_VERSION_MISMATCH", "the dispatched version does not match the authoritative current V2 state version")
	}
	return nil
}

func validateReleaseContextTag(unit releaseconfig.ReleaseUnit, version, dispatchedTag string) *commandFailure {
	tagSpec, err := releaseconfig.NewTagSpec(unit.TagPrefix)
	if err != nil {
		return failureFromMessage("V2_CONTEXT_SOURCE_INVALID", "the selected release unit has an invalid tag policy")
	}
	if tagSpec.Format(version) != dispatchedTag {
		return failureFromMessage("RELEASE_TAG_MISMATCH", "the dispatched tag does not match the selected release unit and version")
	}
	return nil
}

func validateReleaseObjectIDForFormat(objectID string, objectFormat GitObjectFormat) *commandFailure {
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
