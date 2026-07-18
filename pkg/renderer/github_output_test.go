package renderer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestGitHubOutputUsesExplicitDestinationAndDeclaredOrder(t *testing.T) {
	destination := newGitHubOutputDestination(t)
	response := githubOutputTestResponse()
	var stdout bytes.Buffer

	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub, GitHubOutputFile: destination}, &stdout); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("GitHub output contaminated stdout: %q", stdout.String())
	}
	want := "unit=api\nversion=2.4.0\nhead_matches=true\nempty=\nunicode=猫 service\n"
	if got := readGitHubOutputDestination(t, destination); got != want {
		t.Fatalf("GitHub output:\nwant %q\n got %q", want, got)
	}
}

func TestGitHubOutputSafelyEncodesNewlinesCarriageReturnsAndDelimiterLines(t *testing.T) {
	destination := newGitHubOutputDestination(t)
	response := &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"newline":  "first\nNEKO_OUTPUT_NEWLINE\r\nlast",
			"carriage": "left\rright",
		},
		GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{
			{Name: "newline", DataKey: "newline"},
			{Name: "carriage", DataKey: "carriage"},
		}},
	}

	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub, GitHubOutputFile: destination}, &bytes.Buffer{}); err != nil {
		t.Fatalf("RenderWithOptionsTo: %v", err)
	}
	got := readGitHubOutputDestination(t, destination)
	if !strings.Contains(got, "newline<<NEKO_OUTPUT_NEWLINE_1\n") || !strings.Contains(got, "\nNEKO_OUTPUT_NEWLINE_1\n") {
		t.Fatalf("delimiter collision was not escaped deterministically:\n%q", got)
	}
	if !strings.Contains(got, "carriage<<NEKO_OUTPUT_CARRIAGE\nleft\rright\nNEKO_OUTPUT_CARRIAGE\n") {
		t.Fatalf("carriage return was not encoded as a multiline value:\n%q", got)
	}
}

func TestGitHubOutputRejectsUnsafeOrAmbiguousDeclarationsBeforeWriting(t *testing.T) {
	tests := []struct {
		response *plugin.Response
		name     string
	}{
		{name: "missing declaration", response: &plugin.Response{Status: "success"}},
		{name: "invalid name", response: &plugin.Response{Status: "success", Data: map[string]any{"unit": "api"}, GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{{Name: "Unit-Name", DataKey: "unit"}}}}},
		{name: "repeated name", response: &plugin.Response{Status: "success", Data: map[string]any{"unit": "api"}, GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{{Name: "unit", DataKey: "unit"}, {Name: "unit", DataKey: "unit"}}}}},
		{name: "missing value", response: &plugin.Response{Status: "success", Data: map[string]any{}, GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{{Name: "unit", DataKey: "unit"}}}}},
		{name: "structured value", response: &plugin.Response{Status: "success", Data: map[string]any{"unit": []string{"api"}}, GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{{Name: "unit", DataKey: "unit"}}}}},
		{name: "nul value", response: &plugin.Response{Status: "success", Data: map[string]any{"unit": "api\x00tag"}, GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{{Name: "unit", DataKey: "unit"}}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destination := newGitHubOutputDestination(t)
			if err := os.WriteFile(destination, []byte("existing=yes\n"), 0o600); err != nil {
				t.Fatalf("seed destination: %v", err)
			}
			err := RenderWithOptionsTo(test.response, RenderOptions{Format: FormatGitHub, GitHubOutputFile: destination}, &bytes.Buffer{})
			assertGitHubOutputErrorCode(t, err, GitHubOutputEncodingFailed)
			if got := readGitHubOutputDestination(t, destination); got != "existing=yes\n" {
				t.Fatalf("invalid declaration partially wrote destination: %q", got)
			}
		})
	}
}

func TestGitHubOutputDestinationFailuresAreTypedAndDoNotExposePaths(t *testing.T) {
	response := githubOutputTestResponse()
	t.Run("missing explicit destination", func(t *testing.T) {
		err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub}, &bytes.Buffer{})
		assertGitHubOutputErrorCode(t, err, GitHubOutputDestinationUnavailable)
	})
	t.Run("unavailable explicit destination", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "missing", "github output")
		err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub, GitHubOutputFile: destination}, &bytes.Buffer{})
		assertGitHubOutputErrorCode(t, err, GitHubOutputDestinationUnavailable)
		if strings.Contains(err.Error(), destination) {
			t.Fatalf("output error leaked destination path: %v", err)
		}
	})
}

func TestGitHubOutputIsDeterministicAndDescribeIsRejected(t *testing.T) {
	response := githubOutputTestResponse()
	first := newGitHubOutputDestination(t)
	second := newGitHubOutputDestination(t)
	for _, destination := range []string{first, second} {
		if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub, GitHubOutputFile: destination}, &bytes.Buffer{}); err != nil {
			t.Fatalf("RenderWithOptionsTo: %v", err)
		}
	}
	if got, want := readGitHubOutputDestination(t, first), readGitHubOutputDestination(t, second); got != want {
		t.Fatalf("GitHub output is nondeterministic:\nfirst=%q\nsecond=%q", got, want)
	}
	err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub, GitHubOutputFile: first, Describe: true}, &bytes.Buffer{})
	assertGitHubOutputErrorCode(t, err, GitHubOutputEncodingFailed)
}

func TestGitHubFormatRendersStructuredCommandFailureWithoutDestination(t *testing.T) {
	response := &plugin.Response{
		Status: "error",
		Error:  &plugin.ResponseError{Code: "HEAD_MISMATCH", Message: "checked-out HEAD does not match release_sha"},
	}
	var output bytes.Buffer
	if err := RenderWithOptionsTo(response, RenderOptions{Format: FormatGitHub}, &output); err != nil {
		t.Fatalf("structured failure rendering: %v", err)
	}
	if !strings.Contains(output.String(), "HEAD_MISMATCH") || !strings.Contains(output.String(), "checked-out HEAD") {
		t.Fatalf("structured failure was not rendered: %q", output.String())
	}
}

func githubOutputTestResponse() *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Data: map[string]any{
			"unit":         "api",
			"version":      "2.4.0",
			"head_matches": true,
			"empty":        "",
			"unicode":      "猫 service",
		},
		GitHubOutput: &plugin.GitHubOutput{Fields: []plugin.GitHubOutputField{
			{Name: "unit", DataKey: "unit"},
			{Name: "version", DataKey: "version"},
			{Name: "head_matches", DataKey: "head_matches"},
			{Name: "empty", DataKey: "empty"},
			{Name: "unicode", DataKey: "unicode"},
		}},
	}
}

func newGitHubOutputDestination(t *testing.T) string {
	t.Helper()
	destination := filepath.Join(t.TempDir(), "github output with spaces")
	if err := os.WriteFile(destination, nil, 0o600); err != nil {
		t.Fatalf("create destination: %v", err)
	}
	return destination
}

func readGitHubOutputDestination(t *testing.T, destination string) string {
	t.Helper()
	data, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	return string(data)
}

func assertGitHubOutputErrorCode(t *testing.T, err error, want GitHubOutputErrorCode) {
	t.Helper()
	var outputError *GitHubOutputError
	if !errors.As(err, &outputError) || outputError.Code != want {
		t.Fatalf("error = %#v, want GitHub output code %s", err, want)
	}
}
