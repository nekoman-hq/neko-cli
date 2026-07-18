//nolint:staticcheck // V1 compatibility code intentionally uses deprecated V1 APIs during migration
package validate

//lint:file-ignore SA1019 V1 validation compatibility intentionally uses deprecated V1 APIs during migration

import (
	"os"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestHandleValidateCharacterizesStableResponseBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		arrange      func(t *testing.T)
		request      plugin.Request
		wantStatus   string
		wantCode     string
		wantRenderer string
		wantItems    []map[string]any
	}{
		{
			name:       "missing configuration",
			wantStatus: "error",
			wantCode:   "CONFIG_NOT_FOUND",
		},
		{
			name: "invalid v2 configuration",
			arrange: func(t *testing.T) {
				mustWrite(t, config.V2ConfigPath("."), `{"schemaVersion":2,"units":[]}`)
			},
			wantStatus: "error",
			wantCode:   "CONFIG_INVALID",
		},
		{
			name: "v2 default rows and wrong flag types",
			arrange: func(t *testing.T) {
				mustWrite(t, ".github/workflows/release-default.yml", "name: release default\n")
				writeV2(t, `{"schemaVersion":2,"units":[{
  "id":"default",
  "paths":["**"],
  "tagPrefix":"v",
  "executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-default.yml"}
}]}`, `{"schemaVersion":2,"units":{"default":{"version":"2.2.4"}}}`)
			},
			request:      plugin.Request{Flags: map[string]any{"show": "true", "unit": true}},
			wantStatus:   "success",
			wantRenderer: "table",
			wantItems: []map[string]any{
				{"property": "Configuration", "value": ".neko/release.config.json"},
				{"property": "Schema", "value": "v2"},
				{"property": "Status", "value": "✓ Valid"},
			},
		},
		{
			name: "v1 token remains required",
			arrange: func(t *testing.T) {
				t.Setenv("GITHUB_TOKEN", "")
				mustWrite(t, config.V1FileName, `{
  "project-name": "neko-cli",
  "project-owner": "nekoman-hq",
  "project-type": "backend",
  "release-system": "goreleaser",
  "version": "1.2.3"
}`)
				mustWrite(t, ".goreleaser.yml", "{}")
			},
			wantStatus: "error",
			wantCode:   "VALIDATION_FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withWorkingDirectory(t)
			if tt.arrange != nil {
				tt.arrange(t)
			}

			resp, err := HandleValidate(tt.request)
			if err != nil {
				t.Fatalf("HandleValidate returned Go error: %v", err)
			}
			if resp.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", resp.Status, tt.wantStatus)
			}
			if resp.Metadata.Command != "validate" || resp.Metadata.Timestamp.IsZero() {
				t.Fatalf("unexpected metadata: %#v", resp.Metadata)
			}
			if resp.RendererHint != tt.wantRenderer {
				t.Fatalf("renderer = %q, want %q", resp.RendererHint, tt.wantRenderer)
			}
			if tt.wantCode != "" {
				if resp.Error == nil || resp.Error.Code != tt.wantCode {
					t.Fatalf("error = %#v, want code %q", resp.Error, tt.wantCode)
				}
			}
			if tt.wantItems != nil {
				assertValidateItems(t, resp.Data["items"], tt.wantItems)
			}
		})
	}
}

func TestHandleValidateV2IsReadOnlyAndTokenIndependent(t *testing.T) {
	withWorkingDirectory(t)
	t.Setenv("GITHUB_TOKEN", "")
	mustWrite(t, ".github/workflows/release-default.yml", "name: release default\n")
	writeV2(t, `{"schemaVersion":2,"units":[{
  "id":"default",
  "paths":["**"],
  "tagPrefix":"v",
  "executor":{"type":"goreleaser","delivery":"github-actions","workflow":".github/workflows/release-default.yml"}
}]}`, `{"schemaVersion":2,"units":{"default":{"version":"2.2.4"}}}`)

	configBefore := readValidateFile(t, config.V2ConfigPath("."))
	stateBefore := readValidateFile(t, config.V2StatePath("."))
	resp, err := HandleValidate(plugin.Request{Flags: map[string]any{"show": true}})
	if err != nil || resp.Status != "success" {
		t.Fatalf("HandleValidate = (%#v, %v), want success", resp, err)
	}
	if got := readValidateFile(t, config.V2ConfigPath(".")); got != configBefore {
		t.Fatalf("validate mutated config\nbefore: %q\nafter:  %q", configBefore, got)
	}
	if got := readValidateFile(t, config.V2StatePath(".")); got != stateBefore {
		t.Fatalf("validate mutated state\nbefore: %q\nafter:  %q", stateBefore, got)
	}
}

func assertValidateItems(t *testing.T, got any, want []map[string]any) {
	t.Helper()
	items, ok := got.([]map[string]any)
	if !ok {
		t.Fatalf("items type = %T, want []map[string]any", got)
	}
	if len(items) != len(want) {
		t.Fatalf("items = %#v, want %#v", items, want)
	}
	for i := range want {
		if items[i]["property"] != want[i]["property"] || items[i]["value"] != want[i]["value"] {
			t.Fatalf("item %d = %#v, want %#v", i, items[i], want[i])
		}
	}
}

func readValidateFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
