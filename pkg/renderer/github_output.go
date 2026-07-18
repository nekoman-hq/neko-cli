package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// GitHubOutputErrorCode is a stable machine-readable Core output failure code.
type GitHubOutputErrorCode string

const (
	// GitHubOutputDestinationUnavailable means no writable explicit command-file
	// destination was available.
	GitHubOutputDestinationUnavailable GitHubOutputErrorCode = "GITHUB_OUTPUT_DESTINATION_UNAVAILABLE"
	// GitHubOutputEncodingFailed means the response declaration or value could
	// not be encoded safely.
	GitHubOutputEncodingFailed GitHubOutputErrorCode = "GITHUB_OUTPUT_ENCODING_FAILED"
)

// GitHubOutputError reports a stable GitHub Actions output failure without
// exposing filesystem internals.
type GitHubOutputError struct {
	Code    GitHubOutputErrorCode
	Message string
}

func (outputError *GitHubOutputError) Error() string {
	return string(outputError.Code) + ": " + outputError.Message
}

var githubOutputNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func renderGitHubOutput(response *plugin.Response, destination string) error {
	if destination == "" {
		return newGitHubOutputError(GitHubOutputDestinationUnavailable, "an explicit --github-output-file destination is required")
	}
	encoded, err := encodeGitHubOutput(response)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(destination, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return newGitHubOutputError(GitHubOutputDestinationUnavailable, "the explicit GitHub output destination is unavailable")
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return newGitHubOutputError(GitHubOutputEncodingFailed, "GitHub output could not be written safely")
	}
	if err := file.Close(); err != nil {
		return newGitHubOutputError(GitHubOutputEncodingFailed, "GitHub output could not be finalized safely")
	}
	return nil
}

func encodeGitHubOutput(response *plugin.Response) ([]byte, error) {
	if response.GitHubOutput == nil || len(response.GitHubOutput.Fields) == 0 {
		return nil, newGitHubOutputError(GitHubOutputEncodingFailed, "the plugin response does not declare GitHub output fields")
	}
	var encoded bytes.Buffer
	seen := make(map[string]struct{}, len(response.GitHubOutput.Fields))
	for _, field := range response.GitHubOutput.Fields {
		if !githubOutputNamePattern.MatchString(field.Name) || strings.TrimSpace(field.DataKey) == "" || field.DataKey != strings.TrimSpace(field.DataKey) {
			return nil, newGitHubOutputError(GitHubOutputEncodingFailed, "the plugin response contains an invalid GitHub output declaration")
		}
		if _, duplicate := seen[field.Name]; duplicate {
			return nil, newGitHubOutputError(GitHubOutputEncodingFailed, "the plugin response repeats a GitHub output name")
		}
		value, present := response.Data[field.DataKey]
		if !present {
			return nil, newGitHubOutputError(GitHubOutputEncodingFailed, "a declared GitHub output value is missing")
		}
		scalar, scalarErr := githubOutputScalar(value)
		if scalarErr != nil || strings.ContainsRune(scalar, '\x00') {
			return nil, newGitHubOutputError(GitHubOutputEncodingFailed, "a declared GitHub output value cannot be encoded safely")
		}
		seen[field.Name] = struct{}{}
		writeGitHubOutputField(&encoded, field.Name, scalar)
	}
	return encoded.Bytes(), nil
}

func githubOutputScalar(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int8:
		return strconv.FormatInt(int64(typed), 10), nil
	case int16:
		return strconv.FormatInt(int64(typed), 10), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64), nil
	case json.Number:
		return typed.String(), nil
	default:
		return "", fmt.Errorf("unsupported GitHub output scalar %T", value)
	}
}

func writeGitHubOutputField(encoded *bytes.Buffer, name, value string) {
	if !strings.ContainsAny(value, "\r\n") {
		_, _ = fmt.Fprintf(encoded, "%s=%s\n", name, value)
		return
	}
	delimiter := githubOutputDelimiter(name, value)
	_, _ = fmt.Fprintf(encoded, "%s<<%s\n", name, delimiter)
	_, _ = encoded.WriteString(value)
	if !strings.HasSuffix(value, "\n") {
		_ = encoded.WriteByte('\n')
	}
	_, _ = fmt.Fprintf(encoded, "%s\n", delimiter)
}

func githubOutputDelimiter(name, value string) string {
	base := "NEKO_OUTPUT_" + strings.ToUpper(name)
	delimiter := base
	for suffix := 1; githubOutputContainsDelimiter(value, delimiter); suffix++ {
		delimiter = base + "_" + strconv.Itoa(suffix)
	}
	return delimiter
}

func githubOutputContainsDelimiter(value, delimiter string) bool {
	for _, line := range strings.Split(value, "\n") {
		if strings.TrimSuffix(line, "\r") == delimiter {
			return true
		}
	}
	return false
}

func newGitHubOutputError(code GitHubOutputErrorCode, message string) *GitHubOutputError {
	return &GitHubOutputError{Code: code, Message: message}
}
