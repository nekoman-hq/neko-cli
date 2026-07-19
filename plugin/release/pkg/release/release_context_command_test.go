package release

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/pkg/renderer"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func TestParseReleaseContextValidationRequestUsesTypedFlagsAndExplicitRoot(t *testing.T) {
	root := releaseContextCommandRoot(t)
	request := plugin.Request{Flags: map[string]any{
		"unit":        "api",
		"version":     "2.4.0",
		"tag":         "api/v2.4.0",
		"release-sha": strings.Repeat("a", 40),
	}}
	want := ReleaseContextValidationRequest{
		RepositoryRoot: root.Path(),
		UnitID:         "api",
		Version:        "2.4.0",
		Tag:            "api/v2.4.0",
		ReleaseSHA:     strings.Repeat("a", 40),
	}
	if got := ParseReleaseContextValidationRequest(root, request); got != want {
		t.Fatalf("request = %#v, want %#v", got, want)
	}

	malformed := ParseReleaseContextValidationRequest(root, plugin.Request{Flags: map[string]any{
		"unit": 42, "version": true, "tag": []string{"tag"}, "release-sha": nil,
	}})
	if malformed.RepositoryRoot != root.Path() || malformed.UnitID != "" || malformed.Version != "" || malformed.Tag != "" || malformed.ReleaseSHA != "" {
		t.Fatalf("malformed transport values escaped typed parsing: %#v", malformed)
	}
}

func TestReleaseContextValidationCommandHandlerMapsSuccessAtBoundary(t *testing.T) {
	timestamp := time.Date(2026, time.July, 18, 15, 0, 0, 0, time.UTC)
	root := releaseContextCommandRoot(t)
	validator := &recordingReleaseContextValidator{result: validatedReleaseContextFixture()}
	handler := releaseContextValidationCommandHandler{validator: validator, clock: fixedReleaseClock{timestamp}, root: root}
	request := plugin.Request{Flags: map[string]any{
		"unit": "api", "version": "2.4.0", "tag": "api/v2.4.0", "release-sha": strings.Repeat("a", 40),
	}}

	response, err := handler.Handle(context.Background(), request)
	if err != nil {
		t.Fatalf("Handle returned Go error: %v", err)
	}
	if validator.calls != 1 || validator.request.RepositoryRoot != root.Path() || validator.request.UnitID != "api" {
		t.Fatalf("validator calls=%d request=%#v", validator.calls, validator.request)
	}
	if response.Status != "success" || response.ExitCode != 0 || response.Metadata.Command != releaseContextValidationCommandName || !response.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("response envelope = %#v", response)
	}
}

func TestReleaseContextValidationCommandHandlerReturnsStructuredFailureWithNilGoError(t *testing.T) {
	timestamp := time.Date(2026, time.July, 18, 15, 1, 0, 0, time.UTC)
	failure := failureFromMessage("HEAD_MISMATCH", "checked-out HEAD does not match release_sha")
	handler := releaseContextValidationCommandHandler{
		validator: &recordingReleaseContextValidator{failure: failure},
		clock:     fixedReleaseClock{timestamp},
		root:      releaseContextCommandRoot(t),
	}

	response, err := handler.Handle(context.Background(), plugin.Request{})
	if err != nil {
		t.Fatalf("expected structured nil-Go-error response, got %v", err)
	}
	if response.Status != "error" || response.ExitCode != 1 || response.Error == nil || response.Error.Code != "HEAD_MISMATCH" || response.Error.Message != failure.Message {
		t.Fatalf("failure response = %#v", response)
	}
}

func TestValidatedReleaseContextResponseHasStableTypedMachineSchema(t *testing.T) {
	timestamp := time.Date(2026, time.July, 18, 15, 2, 0, 0, time.UTC)
	response := MapValidatedReleaseContext(validatedReleaseContextFixture(), timestamp)
	wantKeys := []string{
		"valid", "unit", "display_name", "version", "tag_prefix", "tag", "release_sha", "working_directory",
		"executor", "delivery", "workflow", "git_object_format", "head_matches", "tag_target_matches",
	}
	if got := sortedMapKeys(response.Data); !reflect.DeepEqual(got, sortedStrings(wantKeys)) {
		t.Fatalf("JSON data keys = %#v, want %#v", got, sortedStrings(wantKeys))
	}
	for _, key := range []string{"valid", "head_matches", "tag_target_matches"} {
		if value, ok := response.Data[key].(bool); !ok || !value {
			t.Fatalf("%s = %#v, want typed true", key, response.Data[key])
		}
	}
	if response.Data["git_object_format"] != "sha1" || response.RendererHint != "table" {
		t.Fatalf("machine response = %#v", response)
	}

	propertyKeys := make([]string, 0, len(response.PresentationProperties.Properties))
	for _, property := range response.PresentationProperties.Properties {
		propertyKeys = append(propertyKeys, property.Key)
	}
	if !reflect.DeepEqual(propertyKeys, wantKeys) {
		t.Fatalf("human property order = %#v, want %#v", propertyKeys, wantKeys)
	}
	wantGitHubKeys := []string{"unit", "display_name", "version", "tag_prefix", "tag", "release_sha", "working_directory", "executor", "delivery", "workflow"}
	githubKeys := make([]string, 0, len(response.GitHubOutput.Fields))
	for _, field := range response.GitHubOutput.Fields {
		githubKeys = append(githubKeys, field.Name)
		if field.Name != field.DataKey {
			t.Fatalf("GitHub field maps %q to unexpected data key %q", field.Name, field.DataKey)
		}
	}
	if !reflect.DeepEqual(githubKeys, wantGitHubKeys) {
		t.Fatalf("GitHub output order = %#v, want %#v", githubKeys, wantGitHubKeys)
	}
}

func TestValidatedReleaseContextReadableJSONAndGitHubOutputs(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "secret-value-that-must-not-appear")
	response := MapValidatedReleaseContext(validatedReleaseContextFixture(), time.Time{})

	var human bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatTable, &human); err != nil {
		t.Fatalf("human render: %v", err)
	}
	plain := ansi.Strip(human.String())
	for _, expected := range []string{"Release context valid", "Unit", "api", "Version", "2.4.0", "HEAD matches", "true", "Tag target matches"} {
		if !strings.Contains(plain, expected) {
			t.Fatalf("human output omitted %q:\n%s", expected, plain)
		}
	}

	var publicJSON bytes.Buffer
	if err := renderer.RenderTo(response, renderer.FormatJSON, &publicJSON); err != nil {
		t.Fatalf("JSON render: %v", err)
	}
	var decoded struct {
		Data   map[string]any `json:"data"`
		Status string         `json:"status"`
	}
	if err := json.Unmarshal(publicJSON.Bytes(), &decoded); err != nil {
		t.Fatalf("decode public JSON: %v", err)
	}
	if decoded.Status != "success" || decoded.Data["head_matches"] != true || strings.Contains(publicJSON.String(), "human_properties") || strings.Contains(publicJSON.String(), "github_output") {
		t.Fatalf("public JSON contract changed:\n%s", publicJSON.String())
	}

	destination := filepath.Join(t.TempDir(), "github output with spaces")
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatalf("create output destination: %v", err)
	}
	var stdout bytes.Buffer
	if err := renderer.RenderWithOptionsTo(response, renderer.RenderOptions{Format: renderer.FormatGitHub, GitHubOutputFile: destination}, &stdout); err != nil {
		t.Fatalf("GitHub render: %v", err)
	}
	encoded, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read GitHub output: %v", err)
	}
	want := "unit=api\ndisplay_name=API μservice\nversion=2.4.0\ntag_prefix=api/v\ntag=api/v2.4.0\n" +
		"release_sha=" + strings.Repeat("a", 40) + "\nworking_directory=services/api with spaces\nexecutor=jreleaser\n" +
		"delivery=github-actions\nworkflow=.github/workflows/release.yml\n"
	if stdout.Len() != 0 || string(encoded) != want || strings.Contains(string(encoded), "secret-value-that-must-not-appear") {
		t.Fatalf("GitHub output stdout=%q\nwant=%q\n got=%q", stdout.String(), want, encoded)
	}
}

func validatedReleaseContextFixture() *ValidatedReleaseContext {
	return &ValidatedReleaseContext{
		UnitID:           "api",
		DisplayName:      "API μservice",
		Version:          "2.4.0",
		TagPrefix:        "api/v",
		Tag:              "api/v2.4.0",
		ReleaseSHA:       strings.Repeat("a", 40),
		WorkingDirectory: "services/api with spaces",
		Executor:         "jreleaser",
		Delivery:         "github-actions",
		Workflow:         ".github/workflows/release.yml",
		GitObjectFormat:  GitObjectFormatSHA1,
		HeadMatches:      true,
		TagTargetMatches: true,
	}
}

type recordingReleaseContextValidator struct {
	result  *ValidatedReleaseContext
	failure *CommandFailure
	request ReleaseContextValidationRequest
	calls   int
}

func (validator *recordingReleaseContextValidator) Validate(_ context.Context, request ReleaseContextValidationRequest) (*ValidatedReleaseContext, *CommandFailure) {
	validator.calls++
	validator.request = request
	return validator.result, validator.failure
}

func releaseContextCommandRoot(t *testing.T) workspace.RepositoryRoot {
	t.Helper()
	root, err := workspace.ValidateRepositoryRoot(t.TempDir())
	if err != nil {
		t.Fatalf("ValidateRepositoryRoot: %v", err)
	}
	return root
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return sortedStrings(keys)
}

func sortedStrings(values []string) []string {
	copyValues := append([]string(nil), values...)
	slices.Sort(copyValues)
	return copyValues
}
