package plugin

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/nekoman-hq/neko-cli/pkg/presentation"
)

// responseWire isolates the established plugin protocol names. The human_*
// JSON tags remain stable so installed plugins continue to interoperate with
// newer Core binaries.
//
//nolint:govet // Field order preserves the stable response protocol order.
type responseWire struct {
	Status                 string                   `json:"status"`
	Metadata               ResponseMetadata         `json:"metadata"`
	Data                   map[string]any           `json:"data,omitempty"`
	Error                  *ResponseError           `json:"error,omitempty"`
	RendererHint           string                   `json:"renderer_hint,omitempty"`
	Logs                   []LogEntry               `json:"logs,omitempty"`
	PresentationTable      *presentation.Table      `json:"human_table,omitempty"`
	PresentationProperties *presentation.Properties `json:"human_properties,omitempty"`
	PresentationText       *presentation.Text       `json:"human_text,omitempty"`
	GitHubOutput           *GitHubOutput            `json:"github_output,omitempty"`
	ExitCode               *int                     `json:"exit_code,omitempty"`
}

// MarshalJSON preserves the established plugin wire tags while accepting both
// canonical and deprecated Go response fields.
func (response Response) MarshalJSON() ([]byte, error) {
	table, err := response.TablePresentation()
	if err != nil {
		return nil, err
	}
	properties, err := response.PropertiesPresentation()
	if err != nil {
		return nil, err
	}
	text, err := response.TextPresentation()
	if err != nil {
		return nil, err
	}
	var exitCode *int
	if code, present := response.ExplicitExitCode(); present {
		exitCode = &code
	}
	return json.Marshal(responseWire{
		Status:                 response.Status,
		Metadata:               response.Metadata,
		Data:                   response.Data,
		Error:                  response.Error,
		RendererHint:           response.RendererHint,
		Logs:                   response.Logs,
		PresentationTable:      table,
		PresentationProperties: properties,
		PresentationText:       text,
		GitHubOutput:           response.GitHubOutput,
		ExitCode:               exitCode,
	})
}

// UnmarshalJSON decodes the established plugin wire tags into canonical fields
// and mirrors the same pointers into deprecated fields for source compatibility.
func (response *Response) UnmarshalJSON(data []byte) error {
	var wire responseWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*response = Response{
		Status:                 wire.Status,
		Metadata:               wire.Metadata,
		Data:                   wire.Data,
		Error:                  wire.Error,
		RendererHint:           wire.RendererHint,
		Logs:                   wire.Logs,
		PresentationTable:      wire.PresentationTable,
		PresentationProperties: wire.PresentationProperties,
		PresentationText:       wire.PresentationText,
		HumanTable:             wire.PresentationTable,
		HumanProperties:        wire.PresentationProperties,
		HumanText:              wire.PresentationText,
		GitHubOutput:           wire.GitHubOutput,
	}
	if wire.ExitCode != nil {
		response.SetExitCode(*wire.ExitCode)
	}
	return nil
}

// TablePresentation resolves the canonical table declaration and its
// deprecated response-field counterpart.
func (response *Response) TablePresentation() (*presentation.Table, error) {
	if response == nil {
		return nil, nil
	}
	return resolvePresentationField("table", response.PresentationTable, response.HumanTable)
}

// PropertiesPresentation resolves the canonical properties declaration and
// its deprecated response-field counterpart.
func (response *Response) PropertiesPresentation() (*presentation.Properties, error) {
	if response == nil {
		return nil, nil
	}
	return resolvePresentationField("properties", response.PresentationProperties, response.HumanProperties)
}

// TextPresentation resolves the canonical text declaration and its deprecated
// response-field counterpart.
func (response *Response) TextPresentation() (*presentation.Text, error) {
	if response == nil {
		return nil, nil
	}
	return resolvePresentationField("text", response.PresentationText, response.HumanText)
}

func resolvePresentationField[T any](name string, canonical, deprecated *T) (*T, error) {
	if canonical == nil {
		return deprecated, nil
	}
	if deprecated == nil || reflect.DeepEqual(canonical, deprecated) {
		return canonical, nil
	}
	return nil, fmt.Errorf("conflicting canonical and deprecated %s presentation fields", name)
}
