package renderer

import (
	"fmt"
	"io"
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func renderTextPresentation(response *plugin.Response, writer io.Writer) (bool, error) {
	if response.PresentationText == nil {
		return false, nil
	}
	if response.PresentationText.Content == "" {
		return true, fmt.Errorf("text presentation declaration is empty")
	}
	content := response.PresentationText.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	_, err := io.WriteString(writer, content)
	return true, err
}
