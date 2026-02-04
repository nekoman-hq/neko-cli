package plugin

import "time"

// Request is the input to the Plugin
type Request struct {
	Command string         `json:"command"`
	Args    []string       `json:"args"`
	Flags   map[string]any `json:"flags"`
	Context Context        `json:"context"`
}

// Context contains execution context information
type Context struct {
	WorkingDir string `json:"working_dir"`
	User       string `json:"user"`
	Verbose    bool   `json:"verbose"`
}

// Response is the output from the Plugin
type Response struct {
	Status       string           `json:"status"`
	Metadata     ResponseMetadata `json:"metadata"`
	Data         map[string]any   `json:"data,omitempty"`
	Error        *ResponseError   `json:"error,omitempty"`
	RendererHint string           `json:"renderer_hint,omitempty"`
	Logs         []LogEntry       `json:"logs,omitempty"`
}

type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"` // "info", "verbose", "warn", "error"
	Category  string `json:"category"`
	Message   string `json:"message"`
}

type ResponseMetadata struct {
	Timestamp time.Time `json:"timestamp"`
	Plugin    string    `json:"plugin"`
	Version   string    `json:"version"`
	Command   string    `json:"command"`
}

type ResponseError struct {
	Details map[string]any `json:"details,omitempty"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
}

// Plugin is the interface that all plugins must implement
type Plugin interface {
	// Execute executes the plugin command with the given request
	Execute(req Request) (*Response, error)

	// Manifest returns the plugin manifest
	Manifest() Manifest
}

// Manifest describes the plugin
type Manifest struct {
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Description   string    `json:"description"`
	Author        string    `json:"author"`
	Commands      []Command `json:"commands"`
	RendererTypes []string  `json:"renderer_types"`
}

// Command describes a plugin command
type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Outputs     []string `json:"outputs"`
	Flags       []Flag   `json:"flags,omitempty"`
}

// Flag describes a command flag
type Flag struct {
	Default     any    `json:"default,omitempty"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "bool", "int"
	Description string `json:"description"`
	Required    bool   `json:"required"`
}
