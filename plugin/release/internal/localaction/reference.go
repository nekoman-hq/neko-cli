package localaction

import (
	"path"
	"strings"
)

// Failure codes report why one repository-local action reference could not be
// expanded. They are stable identifiers consumed by workflow diagnostics.
const (
	FailureReferenceInvalid  = "LOCAL_ACTION_REFERENCE_INVALID"
	FailureMissing           = "LOCAL_ACTION_MISSING"
	FailurePathEscape        = "LOCAL_ACTION_PATH_ESCAPE"
	FailureDefinitionInvalid = "LOCAL_ACTION_DEFINITION_INVALID"
	FailureRecursive         = "LOCAL_ACTION_RECURSIVE"
)

// MaxDepth bounds nested repository-local composite action expansion.
const MaxDepth = 4

// localActionReference reports the raw path of a repository-local `uses`
// value. Remote forms such as `owner/repository@ref`, `docker://…`, and URLs
// are not repository-local and are never expanded.
func localActionReference(uses string) (string, bool) {
	trimmed := strings.TrimSpace(uses)
	switch {
	case trimmed == "":
		return "", false
	case strings.HasPrefix(trimmed, "./"),
		strings.HasPrefix(trimmed, "../"),
		strings.HasPrefix(trimmed, "/"),
		strings.HasPrefix(trimmed, `.\`),
		strings.HasPrefix(trimmed, `..\`):
		return trimmed, true
	default:
		return "", false
	}
}

// repositoryRelativeActionDirectory normalizes a repository-local reference to
// a repository-relative slash path. Absolute paths, parent traversal, and
// Windows-style separators are rejected.
func repositoryRelativeActionDirectory(reference string) (string, bool) {
	if strings.ContainsAny(reference, `\`) || strings.HasPrefix(reference, "/") {
		return "", false
	}
	for _, segment := range strings.Split(reference, "/") {
		if segment == ".." {
			return "", false
		}
	}
	clean := path.Clean(strings.TrimPrefix(reference, "./"))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}
