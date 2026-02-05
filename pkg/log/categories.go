package log

/*
@Author     Benjamin Senekowitsch
@Contact    senekowitsch@nekoman.at
@Since      20.12.2025
*/

// Category represents a logical grouping for log messages.
// Categories help organize log output and apply consistent coloring
// to related operations throughout the CLI lifecycle.
type Category string

const (
	// Init represents initialization and startup operations.
	// Used for logging during CLI bootstrap, plugin discovery, and configuration loading.
	Init Category = "init"

	// Config represents configuration-related operations.
	// Used for logging configuration parsing, validation, and merging.
	Config Category = "config"

	// Preflight represents pre-execution validation and checks.
	// Used for logging prerequisite checks, dependency validation, and sanity tests
	// that run before the main command execution.
	Preflight Category = "pre-flight"

	// Guard represents security and access control operations.
	// Used for logging authentication, authorization, and permission checks.
	Guard Category = "guard"

	// Exec represents command execution operations.
	// Used for logging the actual execution of commands, plugins, and subprocesses.
	Exec Category = "exec"
)

// categoryColors maps each Category to its associated ANSI color code.
// This ensures consistent, recognizable coloring for each category type
// throughout the CLI output.
//
// Color assignments:
//   - Init: Bright Yellow (startup operations)
//   - Config: Bright Cyan (configuration handling)
//   - Preflight: Bright Yellow (validation checks)
//   - Guard: Bright Blue (security operations)
//   - Exec: Bright Green (execution operations)
var categoryColors = map[Category]string{
	Init:      ColorBrightYellow,
	Config:    ColorBrightCyan,
	Preflight: ColorBrightYellow,
	Guard:     ColorBrightBlue,
	Exec:      ColorBrightGreen,
}
