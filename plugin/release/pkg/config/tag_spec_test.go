package config

import "testing"

func TestTagSpecFormatParseAndMatch(t *testing.T) {
	tests := []struct {
		prefix  string
		version string
		tag     string
	}{
		{"v", "2.2.4", "v2.2.4"},
		{"api/v", "0.1.0", "api/v0.1.0"},
		{"web/v", "1.4.2-rc.1", "web/v1.4.2-rc.1"},
		{"mobile/v", "1.0.0", "mobile/v1.0.0"},
	}

	for _, tt := range tests {
		spec, err := NewTagSpec(tt.prefix)
		if err != nil {
			t.Fatalf("NewTagSpec(%q): %v", tt.prefix, err)
		}
		if got := spec.Format(tt.version); got != tt.tag {
			t.Fatalf("Format(%q) = %q, want %q", tt.version, got, tt.tag)
		}
		version, ok := spec.Parse(tt.tag)
		if !ok || version != tt.version {
			t.Fatalf("Parse(%q) = %q/%v, want %q/true", tt.tag, version, ok, tt.version)
		}
		if !spec.Matches(tt.tag) {
			t.Fatalf("Matches(%q) = false", tt.tag)
		}
	}
}

func TestTagSpecRejectsInvalidTags(t *testing.T) {
	spec, err := NewTagSpec("api/v")
	if err != nil {
		t.Fatalf("NewTagSpec: %v", err)
	}
	for _, tag := range []string{
		"api/v1.0.0-extra-non-semver",
		"api/v1.0.0/other",
		"api/vfoo",
		"web/v1.0.0",
		"v1.0.0",
	} {
		if spec.Matches(tag) {
			t.Fatalf("expected %q not to match", tag)
		}
	}
}
