package plugin

import "github.com/nekoman-hq/neko-cli/pkg/presentation"

// HumanTable is retained for source compatibility.
// Deprecated: use presentation.Table.
type HumanTable = presentation.Table

// HumanColumn is retained for source compatibility.
// Deprecated: use presentation.Column.
type HumanColumn = presentation.Column

// HumanProperties is retained for source compatibility.
// Deprecated: use presentation.Properties.
type HumanProperties = presentation.Properties

// HumanProperty is retained for source compatibility.
// Deprecated: use presentation.Property.
type HumanProperty = presentation.Property

// HumanText is retained for source compatibility.
// Deprecated: use presentation.Text.
type HumanText = presentation.Text

// HumanStyleRole is retained for source compatibility.
// Deprecated: use presentation.StyleRole.
type HumanStyleRole = presentation.StyleRole

const (
	// Deprecated: use presentation.StyleDefault.
	HumanStyleDefault = presentation.StyleDefault
	// Deprecated: use presentation.StyleEmphasis.
	HumanStyleEmphasis = presentation.StyleEmphasis
	// Deprecated: use presentation.StyleSuccess.
	HumanStyleSuccess = presentation.StyleSuccess
	// Deprecated: use presentation.StyleWarning.
	HumanStyleWarning = presentation.StyleWarning
	// Deprecated: use presentation.StyleError.
	HumanStyleError = presentation.StyleError
	// Deprecated: use presentation.StyleInfo.
	HumanStyleInfo = presentation.StyleInfo
	// Deprecated: use presentation.StyleMuted.
	HumanStyleMuted = presentation.StyleMuted
)
