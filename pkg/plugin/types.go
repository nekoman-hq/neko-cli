package plugin

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      03.02.2026
*/

import "time"

// Request represents the input data passed to a plugin when executing a command.
// It contains the command to execute, its arguments, flags, and contextual information
// about the execution environment.
type Request struct {
	Command string         `json:"command"` // The name of the command to execute
	Args    []string       `json:"args"`    // Positional arguments for the command
	Flags   map[string]any `json:"flags"`   // Named flags/options for the command
	Context Context        `json:"context"` // Execution context information
}

// Context contains information about the execution environment.
// This provides plugins with necessary context to execute commands appropriately.
type Context struct {
	WorkingDir string `json:"working_dir"` // The current working directory
	User       string `json:"user"`        // The username of the executing user
	Verbose    bool   `json:"verbose"`     // Whether verbose output is enabled
}

// Response represents the output returned by a plugin after executing a command.
// It includes status information, metadata, optional data payload, error details,
// and rendering hints for display.
//
//nolint:govet // Field order preserves the stable response protocol order.
type Response struct {
	Status          string           `json:"status"`                     // Execution status (e.g., "success", "error")
	Metadata        ResponseMetadata `json:"metadata"`                   // Metadata about the response
	Data            map[string]any   `json:"data,omitempty"`             // Optional structured data returned by the plugin
	Error           *ResponseError   `json:"error,omitempty"`            // Error details if the execution failed
	RendererHint    string           `json:"renderer_hint,omitempty"`    // Hint for how to render the response (e.g., "table", "json", "text")
	Logs            []LogEntry       `json:"logs,omitempty"`             // Log entries generated during execution
	HumanTable      *HumanTable      `json:"human_table,omitempty"`      // Optional transport-only declaration for responsive human output
	HumanProperties *HumanProperties `json:"human_properties,omitempty"` // Optional ordered property declaration for one human-facing object
	HumanText       *HumanText       `json:"human_text,omitempty"`       // Optional transport-only preformatted human output
	GitHubOutput    *GitHubOutput    `json:"github_output,omitempty"`    // Optional ordered declaration for GitHub Actions output
	ExitCode        int              `json:"exit_code,omitempty"`        // Optional non-zero Core CLI exit request
}

// HumanTable declares ordered columns for an opt-in responsive human table.
// It is transport metadata and does not change the response Data contract.
type HumanTable struct {
	Columns []HumanColumn `json:"columns"`
}

// HumanColumn declares one human-facing column. Slice order defines display
// order and optional-column priority.
type HumanColumn struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Essential bool   `json:"essential,omitempty"`
}

// HumanProperties declares an ordered property/value view for one result.
// It is transport metadata and does not change the response Data contract.
type HumanProperties struct {
	Properties []HumanProperty `json:"properties"`
}

// HumanProperty declares one human-facing label and either maps it to a
// response Data key or carries a presentation-only value. Slice order defines
// display order. Key and Value are mutually exclusive.
//
//nolint:govet // Field order preserves the stable human-property wire order.
type HumanProperty struct {
	Key   string `json:"key,omitempty"`
	Label string `json:"label"`
	Value any    `json:"value,omitempty"`
}

// HumanText declares preformatted human output for content that must remain
// readable outside a table, such as a generated configuration preview. It is
// transport metadata and does not change the response Data contract.
type HumanText struct {
	Content string `json:"content"`
}

// GitHubOutput declares an ordered set of response Data fields to encode for
// a GitHub Actions command file. It does not select the destination.
type GitHubOutput struct {
	Fields []GitHubOutputField `json:"fields"`
}

// GitHubOutputField maps one stable GitHub Actions output name to a response
// Data key. Slice order defines command-file order.
type GitHubOutputField struct {
	Name    string `json:"name"`
	DataKey string `json:"data_key"`
}

// LogEntry represents a single log message generated during plugin execution.
// Logs can be used for debugging, progress tracking, or informational output.
type LogEntry struct {
	Timestamp string `json:"timestamp"` // ISO 8601 timestamp of when the log was created
	Level     string `json:"level"`     // Log level: "info", "verbose", "warn", or "error"
	Category  string `json:"category"`  // Categorization of the log entry (e.g., "network", "filesystem")
	Message   string `json:"message"`   // The log message content
}

// ResponseMetadata contains contextual information about a plugin response.
// This metadata helps track which plugin and command generated the response.
type ResponseMetadata struct {
	Timestamp time.Time `json:"timestamp"` // When the response was generated
	Plugin    string    `json:"plugin"`    // Name of the plugin that generated the response
	Version   string    `json:"version"`   // Version of the plugin
	Command   string    `json:"command"`   // The command that was executed
}

// ResponseError contains detailed information about an error that occurred
// during plugin execution.
type ResponseError struct {
	Details map[string]any `json:"details,omitempty"` // Additional structured error details
	Code    string         `json:"code"`              // Machine-readable error code
	Message string         `json:"message"`           // Human-readable error message
}

// Plugin is the interface that all neko-cli plugins must implement.
// Plugins extend the CLI's functionality by providing custom commands.
type Plugin interface {
	// Execute runs the plugin command with the given request and returns a response.
	// This is the main entry point for plugin execution.
	//
	// Args:
	//   - req: The request containing command, arguments, flags, and context
	//
	// Returns:
	//   - A pointer to the Response containing execution results
	//   - An error if the plugin execution fails critically
	Execute(req Request) (*Response, error)

	// Manifest returns the plugin's manifest describing its capabilities,
	// commands, and metadata.
	//
	// Returns the Manifest structure for this plugin.
	Manifest() Manifest
}

// Manifest describes a plugin's metadata, capabilities, and available commands.
// It is used for plugin discovery, validation, and documentation.
type Manifest struct {
	Name          string    `json:"name"`           // Unique identifier for the plugin
	Version       string    `json:"version"`        // Semantic version string (e.g., "1.2.3")
	Description   string    `json:"description"`    // Brief description of the plugin's purpose
	Author        string    `json:"author"`         // Plugin author name or organization
	Commands      []Command `json:"commands"`       // List of commands provided by this plugin
	RendererTypes []string  `json:"renderer_types"` // Supported output renderer types (e.g., "table", "json", "yaml")
}

// Command describes a single command provided by a plugin.
// It defines the command's name, description, outputs, and available flags.
type Command struct {
	Name        string   `json:"name"`            // The command name (used to invoke it)
	Description string   `json:"description"`     // Brief description of what the command does
	Outputs     []string `json:"outputs"`         // List of output keys this command can produce
	Flags       []Flag   `json:"flags,omitempty"` // Optional flags/options for this command
}

// Flag describes a command-line flag that can be passed to a plugin command.
// Flags allow users to customize command behavior with named parameters.
type Flag struct {
	Default     any    `json:"default,omitempty"` // Default value if the flag is not provided
	Name        string `json:"name"`              // The flag name (without dashes, e.g., "output" for --output)
	Type        string `json:"type"`              // Data type: "string", "bool", or "int"
	Description string `json:"description"`       // Human-readable description of the flag's purpose
	Required    bool   `json:"required"`          // Whether this flag must be provided
}
