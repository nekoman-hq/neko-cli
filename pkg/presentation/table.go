package presentation

// Table declares ordered columns for an opt-in responsive table. Rows may
// provide a presentation-only projection when the machine-readable data uses a
// different shape. Details may append one responsive property view after the
// table. All fields are transport metadata and do not change response data.
//
//nolint:govet // Field order preserves the stable table-presentation wire order.
type Table struct {
	Columns []Column         `json:"columns"`
	Rows    []map[string]any `json:"rows,omitempty"`
	Details *Properties      `json:"details,omitempty"`
	Title   string           `json:"title,omitempty"`
}

// Column declares one presentation column. Slice order defines display order
// and optional-column priority.
type Column struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	RoleKey   string `json:"role_key,omitempty"`
	Essential bool   `json:"essential,omitempty"`
}
