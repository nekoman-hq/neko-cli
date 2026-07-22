package presentation

// Properties declares an ordered property/value view for one result. Title
// names the result and SectionTitle may name the contained property section.
// It is transport metadata and does not change the response data contract.
//
//nolint:govet // Field order preserves the stable properties-presentation wire order.
type Properties struct {
	Properties   []Property `json:"properties"`
	Title        string     `json:"title,omitempty"`
	SectionTitle string     `json:"section_title,omitempty"`
}

// Property declares one presentation label and either maps it to a response
// data key or carries a presentation-only value. Slice order defines display
// order. Key and Value are mutually exclusive.
//
//nolint:govet // Field order preserves the stable property-presentation wire order.
type Property struct {
	Key        string    `json:"key,omitempty"`
	Label      string    `json:"label"`
	Value      any       `json:"value,omitempty"`
	Role       StyleRole `json:"role,omitempty"`
	Emphasized bool      `json:"emphasized,omitempty"`
	Heading    bool      `json:"heading,omitempty"`
}
