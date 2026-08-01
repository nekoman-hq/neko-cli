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

func TestPermissionGuidanceAddsForceOnlyForSameVersionReinstall(t *testing.T) {
	base := updatePlan{runningExecutable: "/path with spaces/neko", action: ActionUpgrade}
	if got := permissionGuidance(base, true); got == "" || containsStateText(got, "update --force") {
		t.Fatalf("upgrade guidance=%q", got)
	}
	base.action = ActionForcedReinstall
	if got := permissionGuidance(base, true); !containsStateText(got, " update --force") {
		t.Fatalf("forced reinstall guidance=%q", got)
	}
}
