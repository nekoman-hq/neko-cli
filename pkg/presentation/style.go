package presentation

// StyleRole expresses presentation meaning without exposing terminal colors
// or ANSI sequences to plugin response mappers. The empty zero value is
// rendered like StyleDefault for backwards compatibility.
type StyleRole string

const (
	StyleDefault  StyleRole = "default"
	StyleEmphasis StyleRole = "emphasis"
	StyleSuccess  StyleRole = "success"
	StyleWarning  StyleRole = "warning"
	StyleError    StyleRole = "error"
	StyleInfo     StyleRole = "info"
	StyleMuted    StyleRole = "muted"
)
