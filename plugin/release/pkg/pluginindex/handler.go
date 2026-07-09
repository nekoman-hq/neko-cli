package pluginindex

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

const CommandName = "plugin-index"

// HandlePluginIndex generates the public plugin registry index.
func HandlePluginIndex(req plugin.Request) (*plugin.Response, error) {
	outputPath := getFlagString(req.Flags, "output")
	check := getFlagBool(req.Flags, "check")
	pretty := getFlagBoolDefault(req.Flags, "pretty", true)
	repository := getFlagString(req.Flags, "repository")
	if repository == "" {
		repository = DefaultRepository
	}

	if check && outputPath != "" {
		return nil, fmt.Errorf("--check cannot be used with --output")
	}

	index, err := Generate(context.Background(), GenerateOptions{
		Root:       ".",
		Repository: repository,
	})
	if err != nil {
		return nil, err
	}

	if check {
		return response(map[string]any{
			"items": []map[string]any{
				{"property": "Status", "value": "ok"},
				{"property": "Plugins", "value": len(index.Plugins)},
				{"property": "Repository", "value": index.Repository},
			},
		}, ""), nil
	}

	if outputPath != "" {
		if writeErr := writeIndexFile(index, outputPath, pretty); writeErr != nil {
			return nil, writeErr
		}
		return response(map[string]any{
			"items": []map[string]any{
				{"property": "Status", "value": "written"},
				{"property": "Output", "value": outputPath},
				{"property": "Plugins", "value": len(index.Plugins)},
				{"property": "Repository", "value": index.Repository},
			},
		}, ""), nil
	}

	rawJSON, err := renderIndexString(index, pretty)
	if err != nil {
		return nil, err
	}
	return response(map[string]any{"raw": rawJSON}, "raw-json"), nil
}

func writeIndexFile(index *Index, outputPath string, pretty bool) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create plugin index output directory: %w", err)
	}

	var buf bytes.Buffer
	if err := WriteWithOptions(index, &buf, WriteOptions{Pretty: pretty}); err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write plugin index %s: %w", outputPath, err)
	}
	return nil
}

func renderIndexString(index *Index, pretty bool) (string, error) {
	var buf bytes.Buffer
	if err := WriteWithOptions(index, &buf, WriteOptions{Pretty: pretty}); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func response(data map[string]any, rendererHint string) *plugin.Response {
	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   CommandName,
			Timestamp: time.Now(),
		},
		Data:         data,
		RendererHint: rendererHint,
	}
}

func getFlagString(flags map[string]any, name string) string {
	if value, ok := flags[name].(string); ok {
		return value
	}
	return ""
}

func getFlagBool(flags map[string]any, name string) bool {
	if value, ok := flags[name].(bool); ok {
		return value
	}
	return false
}

func getFlagBoolDefault(flags map[string]any, name string, fallback bool) bool {
	if value, ok := flags[name].(bool); ok {
		return value
	}
	return fallback
}
