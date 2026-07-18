package renderer

import (
	"fmt"
	"io"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func renderHumanProperties(response *plugin.Response, writer io.Writer) (bool, error) {
	if response.HumanProperties == nil {
		return false, nil
	}
	declarations := response.HumanProperties.Properties
	if len(declarations) == 0 {
		return true, fmt.Errorf("human property declaration is empty")
	}
	items := make([]map[string]any, 0, len(declarations))
	seen := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		key := strings.TrimSpace(declaration.Key)
		label := strings.TrimSpace(declaration.Label)
		if key == "" || label == "" || key != declaration.Key || label != declaration.Label {
			return true, fmt.Errorf("human property declaration contains an invalid key or label")
		}
		if _, duplicate := seen[key]; duplicate {
			return true, fmt.Errorf("human property declaration repeats data key %q", key)
		}
		value, present := response.Data[key]
		if !present {
			return true, fmt.Errorf("human property data key %q is missing", key)
		}
		seen[key] = struct{}{}
		items = append(items, map[string]any{"property": label, "value": value})
	}
	return true, renderList(items, writer)
}
