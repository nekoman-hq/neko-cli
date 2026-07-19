package log

import "testing"

func TestLoggerANSIColorPaletteAndResetRemainStable(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"reset":          "\x1b[0m",
		"bold":           "\x1b[1m",
		"error":          "\x1b[31m",
		"success":        "\x1b[32m",
		"warning":        "\x1b[33m",
		"information":    "\x1b[36m",
		"secondary":      "\x1b[90m",
		"bright error":   "\x1b[91m",
		"bright success": "\x1b[92m",
	}
	got := map[string]string{
		"reset":          ColorReset,
		"bold":           ColorBold,
		"error":          ColorRed,
		"success":        ColorGreen,
		"warning":        ColorYellow,
		"information":    ColorCyan,
		"secondary":      ColorBrightBlack,
		"bright error":   ColorBrightRed,
		"bright success": ColorBrightGreen,
	}
	for name, wantCode := range want {
		if got[name] != wantCode {
			t.Errorf("%s ANSI code = %q, want %q", name, got[name], wantCode)
		}
	}

	if got := ColorText(ColorYellow, "warning"); got != "\x1b[33mwarning\x1b[0m" {
		t.Fatalf("ColorText reset contract = %q", got)
	}
}
