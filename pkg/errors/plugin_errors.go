// Package errors includes helper functions to display CLI errors or warnings
// for plugins. This package provides functions to create and write plugin.Response
// objects with error information in the format expected by the neko-cli dispatcher.
//
// Note: This package is for plugins only. The core CLI tool uses a different
// errors package for user-facing error display.
package errors

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import (
	"encoding/json"
	"os"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

// PluginName and PluginVersion should be set by plugins before using error functions.
// These values are included in the response metadata to identify the source of errors.
//
// Example usage in a plugin's main():
//
//	errors.PluginName = "release"
//	errors.PluginVersion = "1.2.3"
var (
	PluginName    = "cli"
	PluginVersion = "1.0.0"
)

// WriteError creates an error response, writes it to stdout as JSON, and exits
// with code 1. This is the standard way for plugins to report fatal errors.
//
// The response is written to stdout (not stderr) because the dispatcher reads
// plugin responses from stdout. The JSON format allows the CLI to parse and
// display the error appropriately.
//
// Args:
//   - code: Machine-readable error code (e.g., "INVALID_CONFIG", "FILE_NOT_FOUND")
//   - message: Human-readable error message
//
// Note: This function does not return; it exits the program.
func WriteError(code, message string) {
	resp := plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    PluginName,
			Version:   PluginVersion,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
		},
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}

// WriteErrorWithDetails creates an error response with additional structured details,
// writes it to stdout as JSON, and exits with code 1. Use this when you need to
// provide extra context or data about the error.
//
// Args:
//   - code: Machine-readable error code
//   - message: Human-readable error message
//   - details: Additional structured information about the error (e.g., file paths,
//     line numbers, validation errors)
//
// Example details:
//
//	details := map[string]any{
//	    "file": "/path/to/config.yml",
//	    "line": 42,
//	    "expected": "string",
//	    "got": "number",
//	}
//
// Note: This function does not return; it exits the program.
func WriteErrorWithDetails(code, message string, details map[string]any) {
	resp := plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    PluginName,
			Version:   PluginVersion,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
	_ = json.NewEncoder(os.Stdout).Encode(resp)
	os.Exit(1)
}

// WriteWarning creates a warning response and returns it without exiting.
// This can be used for non-fatal issues that should be brought to the user's
// attention but don't prevent successful execution.
//
// Args:
//   - code: Machine-readable warning code
//   - message: Human-readable warning message
//
// Returns a plugin.Response with status "warning". The caller is responsible
// for writing this to stdout or incorporating it into a larger response.
func WriteWarning(code, message string) *plugin.Response {
	return &plugin.Response{
		Status: "warning",
		Metadata: plugin.ResponseMetadata{
			Plugin:    PluginName,
			Version:   PluginVersion,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// NewErrorResponse creates an error response without writing to stdout or exiting.
// This is useful when you want to return an error from a handler function and
// let the caller decide how to handle it.
//
// Args:
//   - code: Machine-readable error code
//   - message: Human-readable error message
//
// Returns a plugin.Response with status "error" that can be returned to the caller.
func NewErrorResponse(code, message string) *plugin.Response {
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    PluginName,
			Version:   PluginVersion,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
		},
	}
}

// NewErrorResponseWithDetails creates an error response with additional details
// without writing to stdout or exiting. This is useful when you want to return
// a detailed error from a handler function.
//
// Args:
//   - code: Machine-readable error code
//   - message: Human-readable error message
//   - details: Additional structured information about the error
//
// Returns a plugin.Response with status "error" that can be returned to the caller.
func NewErrorResponseWithDetails(code, message string, details map[string]any) *plugin.Response {
	return &plugin.Response{
		Status: "error",
		Metadata: plugin.ResponseMetadata{
			Plugin:    PluginName,
			Version:   PluginVersion,
			Timestamp: time.Now(),
		},
		Error: &plugin.ResponseError{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}
