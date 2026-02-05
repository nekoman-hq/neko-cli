package dispatcher

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// Dispatcher handles the execution of plugins by managing plugin discovery,
// command execution, and response parsing. It acts as the bridge between
// the CLI and the plugin executables.
type Dispatcher struct {
	pluginDir string
}

// NewDispatcher creates a new Dispatcher instance for the given plugin directory.
//
// Args:
//   - pluginDir: The directory where plugins are installed
//
// Returns a configured Dispatcher instance.
func NewDispatcher(pluginDir string) *Dispatcher {
	return &Dispatcher{
		pluginDir: pluginDir,
	}
}

// Dispatch executes a plugin with the given request and returns its response.
// It handles the complete execution lifecycle:
//   - Finding the plugin executable
//   - Marshaling the request to JSON
//   - Executing the plugin with the request as stdin
//   - Capturing stdout (response) and stderr (logs)
//   - Parsing the response and logs
//
// The plugin receives the request via stdin as JSON and must output a JSON
// response to stdout. Logs can be written to stderr in either structured
// format ("15:04:05 [category] message") or plain text.
//
// Args:
//   - ctx: Context for controlling plugin execution lifetime
//   - pluginName: The name of the plugin to execute
//   - req: The request containing command, arguments, flags, and context
//
// Returns:
//   - A pointer to the plugin.Response containing execution results and logs
//   - An error if:
//   - The plugin cannot be found
//   - The request cannot be marshaled
//   - Plugin execution fails without a valid error response
//   - The response cannot be parsed
//
// Note: If a plugin exits with an error but produces a valid JSON response
// on stdout, that response is returned without error, allowing plugins to
// communicate structured error information.
func (d *Dispatcher) Dispatch(ctx context.Context, pluginName string, req plugin.Request) (*plugin.Response, error) {
	pluginPath, err := d.findPlugin(pluginName)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	cmd := exec.CommandContext(ctx, pluginPath)
	cmd.Stdin = bytes.NewReader(reqJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Check if stdout contains a valid JSON response (error response from plugin)
		if stdout.Len() > 0 {
			var resp plugin.Response
			if jsonErr := json.Unmarshal(stdout.Bytes(), &resp); jsonErr == nil {
				// Valid response found, parse logs and return it
				resp.Logs = parseLogOutput(stderr.String())
				return &resp, nil
			}
		}
		return nil, fmt.Errorf("plugin execution failed: %w\nStderr: %s", err, stderr.String())
	}

	var resp plugin.Response
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse plugin response: %w\nOutput: %s", err, stdout.String())
	}

	// Parse stderr as structured logs
	resp.Logs = parseLogOutput(stderr.String())

	return &resp, nil
}

// parseLogOutput converts stderr output into structured log entries.
// It supports both structured log format ("15:04:05 [category] message")
// and plain text lines. Empty lines are ignored.
//
// Args:
//   - stderr: The stderr output from the plugin as a string
//
// Returns a slice of plugin.LogEntry structs. Returns nil if stderr is empty.
func parseLogOutput(stderr string) []plugin.LogEntry {
	if stderr == "" {
		return nil
	}

	var logs []plugin.LogEntry
	scanner := bufio.NewScanner(strings.NewReader(stderr))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry := parseLogLine(line)
		logs = append(logs, entry)
	}

	return logs
}

// parseLogLine attempts to parse a single log line into a structured LogEntry.
// It recognizes the format "15:04:05 [category] message" and extracts the
// components. If the line doesn't match this format, it creates a plain
// log entry with inferred metadata.
//
// Structured format example: "15:04:05 [network] Request completed successfully"
//
// Args:
//   - line: A single line of log output
//
// Returns a plugin.LogEntry with parsed or inferred fields:
//   - Timestamp: Either parsed from line or current time
//   - Level: Inferred from message content (error, warn, verbose, info)
//   - Category: Either parsed from [category] or defaults to "plugin"
//   - Message: The log message content
func parseLogLine(line string) plugin.LogEntry {
	// Try to parse: "15:04:05 [category] message"
	parts := strings.SplitN(line, " ", 3)

	if len(parts) >= 3 && strings.Contains(parts[1], "[") && strings.Contains(parts[1], "]") {

		category := strings.Trim(parts[1], "[]")
		level := inferLogLevel(parts[2])

		return plugin.LogEntry{
			Timestamp: parts[0],
			Level:     level,
			Category:  category,
			Message:   parts[2],
		}
	}

	// Fallback: plain text log
	return plugin.LogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     "info",
		Category:  "plugin",
		Message:   line,
	}
}

// inferLogLevel determines the appropriate log level based on message content.
// It uses keyword detection to classify messages.
//
// Args:
//   - msg: The log message to analyze
//
// Returns one of:
//   - "error": If message contains "error" or "failed" (case-insensitive)
//   - "warn": If message contains "warn" (case-insensitive)
//   - "verbose": If message starts with "V$"
//   - "info": Default level for all other messages
func inferLogLevel(msg string) string {
	msgLower := strings.ToLower(msg)
	if strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed") {
		return "error"
	}
	if strings.Contains(msgLower, "warn") {
		return "warn"
	}
	if strings.HasPrefix(msg, "V$") {
		return "verbose"
	}
	return "info"
}

// findPlugin locates the executable file for a given plugin name.
// It constructs the expected plugin path based on the naming convention:
// {pluginDir}/{name}/plugin-{name}
//
// Args:
//   - name: The name of the plugin to find
//
// Returns:
//   - The full path to the plugin executable
//   - An error if the plugin file does not exist
//
// Example:
//
//	For plugin "release" in "/home/user/.neko/plugins":
//	Returns "/home/user/.neko/plugins/release/plugin-release"
func (d *Dispatcher) findPlugin(name string) (string, error) {
	pluginPath := filepath.Join(d.pluginDir, name, fmt.Sprintf("plugin-%s", name))
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return "", fmt.Errorf("plugin '%s' not found at %s", name, pluginPath)
	}
	return pluginPath, nil
}

// ListPlugins discovers and returns manifests for all installed plugins.
// It scans the plugin directory for subdirectories, reads each manifest.json,
// and returns successfully parsed manifests. Plugins with missing or invalid
// manifests are silently skipped.
//
// Returns:
//   - A slice of plugin.Manifest structs for all valid plugins
//   - An error if the plugin directory cannot be read
//
// Note: Individual manifest parsing errors do not cause the entire operation
// to fail. Only valid manifests are returned in the result.
func (d *Dispatcher) ListPlugins() ([]plugin.Manifest, error) {
	entries, err := os.ReadDir(d.pluginDir)
	if err != nil {
		return nil, err
	}

	var manifests []plugin.Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		manifestPath := filepath.Join(d.pluginDir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var manifest plugin.Manifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}
