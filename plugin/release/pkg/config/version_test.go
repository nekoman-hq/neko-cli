package config

import "testing"

func TestCanonicalReleaseVersionNormalizesAcceptedV2SemVer(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{value: "1.2.3", want: "1.2.3"},
		{value: "v1.2.3", want: "1.2.3"},
		{value: "1.2.3-rc.1+build.5", want: "1.2.3-rc.1+build.5"},
	}
	for _, test := range tests {
		got, err := CanonicalReleaseVersion(test.value)
		if err != nil || got != test.want {
			t.Fatalf("CanonicalReleaseVersion(%q) = %q, %v; want %q", test.value, got, err, test.want)
		}
	}
	if _, err := CanonicalReleaseVersion("not-semver"); err == nil {
		t.Fatal("malformed version unexpectedly passed")
	}
}
