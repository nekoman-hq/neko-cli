package terminal

import "testing"

func TestApplyAlwaysResetsStyledTextAndLeavesPlainTextUnchanged(t *testing.T) {
	t.Parallel()

	if got := Apply(Red+Bold, "not ready"); got != "\x1b[31m\x1b[1mnot ready\x1b[0m" {
		t.Fatalf("styled text = %q", got)
	}
	if got := Apply("", "plain"); got != "plain" {
		t.Fatalf("plain text = %q", got)
	}
}
