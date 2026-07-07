package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
)

var strictTagVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*))?(?:\+[0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*)?$`)

// TagSpec formats and parses tags for one normalized release unit.
type TagSpec struct {
	Prefix string
}

// NewTagSpec creates a tag spec for prefix.
func NewTagSpec(prefix string) (TagSpec, error) {
	if strings.TrimSpace(prefix) == "" {
		return TagSpec{}, fmt.Errorf("tag prefix must not be empty")
	}
	return TagSpec{Prefix: prefix}, nil
}

// Format returns the canonical tag for version.
func (spec TagSpec) Format(version string) string {
	return spec.Prefix + version
}

// Parse extracts a SemVer version from tag when it exactly matches this spec.
func (spec TagSpec) Parse(tag string) (string, bool) {
	if !strings.HasPrefix(tag, spec.Prefix) {
		return "", false
	}
	versionPart := strings.TrimPrefix(tag, spec.Prefix)
	if versionPart == "" || strings.Contains(versionPart, "/") || !strictTagVersionPattern.MatchString(versionPart) {
		return "", false
	}
	version, err := semver.NewVersion(versionPart)
	if err != nil {
		return "", false
	}
	return version.String(), true
}

// Matches reports whether tag belongs to this spec.
func (spec TagSpec) Matches(tag string) bool {
	_, ok := spec.Parse(tag)
	return ok
}

// Pattern returns a Git-compatible glob pattern for coarse tag prefiltering.
func (spec TagSpec) Pattern() string {
	return spec.Prefix + "*"
}
