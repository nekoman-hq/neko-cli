package validate

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestValidationMachineResponseRemainsStableAcrossHumanPresentationChanges(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, time.July, 15, 9, 30, 0, 0, time.UTC)
	tests := []struct {
		name   string
		want   string
		result validationQueryResult
	}{
		{
			name:   "v2 default",
			result: validationQueryResult{SourceFormat: config.SourceFormatV2},
			want:   `{"status":"success","metadata":{"timestamp":"2026-07-15T09:30:00Z","plugin":"release","version":"dev","command":"validate"},"data":{"items":[{"property":"Configuration","value":".neko/release.config.json"},{"property":"Schema","value":"v2"},{"property":"Status","value":"✓ Valid"}]},"renderer_hint":"table"}`,
		},
		{
			name: "v2 show",
			result: validationQueryResult{
				SourceFormat: config.SourceFormatV2,
				Show:         true,
				Units: []config.ReleaseUnit{{
					ID:               "api",
					Version:          "1.2.3",
					WorkingDirectory: ".",
					TagPrefix:        "api/v",
					ExecutorType:     "goreleaser",
					Delivery:         "github-actions",
					Workflow:         ".github/workflows/release-api.yml",
					Paths:            []string{"api/**", "shared/**"},
				}},
			},
			want: `{"status":"success","metadata":{"timestamp":"2026-07-15T09:30:00Z","plugin":"release","version":"dev","command":"validate"},"data":{"items":[{"property":"Schema","value":"v2"},{"property":"Unit api","value":"version=1.2.3 workingDirectory=. tagPrefix=api/v executor=goreleaser delivery=github-actions workflow=.github/workflows/release-api.yml paths=[api/** shared/**]"}]},"renderer_hint":"table"}`,
		},
		{
			name: "v1 show",
			result: validationQueryResult{
				SourceFormat: config.SourceFormatV1,
				Show:         true,
				Legacy: legacyValidationDetails{
					ProjectName:   "neko-cli",
					ProjectOwner:  "nekoman-hq",
					ProjectType:   "backend",
					ReleaseSystem: "goreleaser",
					Version:       "1.2.3",
					UnitID:        "default",
				},
			},
			want: `{"status":"success","metadata":{"timestamp":"2026-07-15T09:30:00Z","plugin":"release","version":"dev","command":"validate"},"data":{"items":[{"property":"Project Name","value":"neko-cli"},{"property":"Project Owner","value":"nekoman-hq"},{"property":"Project Type","value":"backend"},{"property":"Release System","value":"goreleaser"},{"property":"Version","value":"1.2.3"},{"property":"Status","value":"✓ Valid"}]},"renderer_hint":"table"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			response := mapValidationQueryResponse(test.result, nil, timestamp)
			got := validationPublicJSON(t, response)
			if got != test.want {
				t.Fatalf("machine response changed:\nwant %s\n got %s", test.want, got)
			}
		})
	}
}

func TestValidationFailureContractUsesExplicitFailureExit(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, time.July, 15, 9, 30, 0, 0, time.UTC)
	response := mapValidationQueryResponse(validationQueryResult{}, &validationQueryFailure{
		Code:    "CONFIG_NOT_FOUND",
		Message: "No release configuration found",
		Hint:    missingConfigurationHint,
	}, timestamp)

	if code, present := response.ExplicitExitCode(); !present || code != 1 {
		t.Fatalf("explicit exit = (%d, %t), want (1, true)", code, present)
	}
	got := validationPublicJSON(t, response)
	want := `{"status":"error","metadata":{"timestamp":"2026-07-15T09:30:00Z","plugin":"release","version":"dev","command":"validate"},"error":{"details":{"hint":"Run 'neko release init' to create V2 config/state, or 'neko release migrate' to convert an existing V1 config"},"code":"CONFIG_NOT_FOUND","message":"No release configuration found"}}`
	if got != want {
		t.Fatalf("failure response changed:\nwant %s\n got %s", want, got)
	}
}

func validationPublicJSON(t *testing.T, response *plugin.Response) string {
	t.Helper()
	var pretty bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &pretty); err != nil {
		t.Fatalf("render public JSON: %v", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, pretty.Bytes()); err != nil {
		t.Fatalf("compact public JSON: %v", err)
	}
	return compact.String()
}
