package contributors

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/log"
	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/git"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

func HandleContributors() (*plugin.Response, error) {
	log.PluginPrint(log.Exec, "Collecting contributors")

	contributors, _ := git.Contributors()

	items := make([]map[string]any, 0, len(contributors))
	for _, c := range contributors {
		items = append(items, map[string]any{
			"author":  c.Author,
			"commits": c.Commits,
		})
	}

	log.PluginPrint(log.Exec, "Successfully collected contributors")

	return &plugin.Response{
		Status: "success",
		Metadata: plugin.ResponseMetadata{
			Plugin:    metadata.PluginName,
			Version:   metadata.Version,
			Command:   "contributors",
			Timestamp: time.Now(),
		},
		Data: map[string]any{
			"items": items,
		},
	}, nil
}
