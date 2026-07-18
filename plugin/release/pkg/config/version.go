package config

import "github.com/Masterminds/semver/v3"

// CanonicalReleaseVersion parses the Release V2 semantic-version policy and
// returns its normalized representation.
func CanonicalReleaseVersion(value string) (string, error) {
	version, err := semver.NewVersion(value)
	if err != nil {
		return "", err
	}
	return version.String(), nil
}
