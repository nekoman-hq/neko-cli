package update

import (
	"strings"
	"testing"
)

func TestUpdateActionContract(t *testing.T) {
	tests := []struct {
		name      string
		installed string
		available string
		want      Action
		wantError string
		force     bool
	}{
		{name: "upgrade", installed: "1.0.0", available: "1.1.0", want: ActionUpgrade},
		{name: "forced upgrade is unchanged", installed: "1.0.0", available: "1.1.0", force: true, want: ActionUpgrade},
		{name: "already current", installed: "1.1.0", available: "1.1.0", want: ActionAlreadyCurrent},
		{name: "forced reinstall", installed: "1.1.0", available: "1.1.0", force: true, want: ActionForcedReinstall},
		{name: "installed newer", installed: "1.2.0", available: "1.1.0", want: ActionInstalledNewer},
		{name: "forced downgrade refused", installed: "1.2.0", available: "1.1.0", force: true, want: ActionDowngradeRefused, wantError: "force is not a downgrade flag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := classifyUpdateAction(tt.installed, tt.available, tt.force)
			if action != tt.want {
				t.Fatalf("action = %q, want %q", action, tt.want)
			}
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want text %q", err, tt.wantError)
			}
		})
	}
}

func TestDevelopmentBuildContractPrecedesForce(t *testing.T) {
	for _, value := range []string{"dev", "devel", "v1.2.3-1-gabc123", "v1.2.3-dirty"} {
		if !isDevelopmentBuild(value) {
			t.Fatalf("isDevelopmentBuild(%q) = false", value)
		}
	}
}
