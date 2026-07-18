package renderer

import (
	"fmt"
	"io"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func renderHumanText(response *plugin.Response, writer io.Writer) (bool, error) {
	if response.HumanText == nil {
		return false, nil
	}
	if response.HumanText.Content == "" {
		return true, fmt.Errorf("human text declaration is empty")
	}
	content := response.HumanText.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	_, err := io.WriteString(writer, content)
	return true, err
}
