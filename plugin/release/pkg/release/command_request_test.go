package release

import (
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestParseReleaseCommandRequest(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		want  ReleaseCommandRequest
	}{
		{
			name:  "typed flags",
			flags: map[string]any{"unit": "api", "dry-run": true},
			want:  ReleaseCommandRequest{ReleaseType: Minor, UnitID: "api", DryRun: true},
		},
		{
			name: "missing flags use existing defaults",
			want: ReleaseCommandRequest{ReleaseType: Minor},
		},
		{
			name:  "malformed flag types use existing defaults",
			flags: map[string]any{"unit": 42, "dry-run": "true"},
			want:  ReleaseCommandRequest{ReleaseType: Minor},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseReleaseCommandRequest(plugin.Request{Flags: tt.flags}, Minor)
			if got != tt.want {
				t.Fatalf("ParseReleaseCommandRequest() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseResumeCommandRequest(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		want  ResumeCommandRequest
	}{
		{
			name:  "typed flags",
			flags: map[string]any{"unit": "web", "dry-run": true},
			want:  ResumeCommandRequest{UnitID: "web", DryRun: true},
		},
		{name: "missing flags use existing defaults"},
		{
			name:  "malformed flag types use existing defaults",
			flags: map[string]any{"unit": false, "dry-run": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseResumeCommandRequest(plugin.Request{Flags: tt.flags})
			if got != tt.want {
				t.Fatalf("ParseResumeCommandRequest() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
