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

func TestParsePlanCommandRequest(t *testing.T) {
	tests := []struct {
		name     string
		flags    map[string]any
		want     PlanCommandRequest
		wantCode string
	}{
		{
			name:  "typed flags",
			flags: map[string]any{"change": "minor", "unit": "api"},
			want:  PlanCommandRequest{ReleaseType: Minor, UnitID: "api"},
		},
		{
			name:     "missing change fails",
			wantCode: "INVALID_RELEASE_CHANGE",
		},
		{
			name:     "malformed change type fails",
			flags:    map[string]any{"change": false},
			wantCode: "INVALID_RELEASE_CHANGE",
		},
		{
			name:     "unknown change fails",
			flags:    map[string]any{"change": "build"},
			wantCode: "INVALID_RELEASE_CHANGE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, failure := ParsePlanCommandRequest(plugin.Request{Flags: tt.flags})
			if tt.wantCode != "" {
				if failure == nil || failure.Code != tt.wantCode {
					t.Fatalf("failure=%#v, want %s", failure, tt.wantCode)
				}
				return
			}
			if failure != nil {
				t.Fatalf("unexpected failure: %#v", failure)
			}
			if got != tt.want {
				t.Fatalf("ParsePlanCommandRequest() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
