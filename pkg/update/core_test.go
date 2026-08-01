package update

import "testing"

func TestNormalizeVersion(t *testing.T) {
	for input, expected := range map[string]string{"1.2.3": "v1.2.3", "v1.2.3": "v1.2.3"} {
		if got := normalizeVersion(input); got != expected {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestShellQuoteForPermissionGuidance(t *testing.T) {
	if got := shellQuote("/path with spaces/neko"); got != "'/path with spaces/neko'" {
		t.Fatalf("shellQuote = %q", got)
	}
	if got := shellQuote("/usr/local/bin/neko"); got != "/usr/local/bin/neko" {
		t.Fatalf("shellQuote = %q", got)
	}
}
