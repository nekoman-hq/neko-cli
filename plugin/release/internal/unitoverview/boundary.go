package unitoverview

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type unitOverviewClock interface {
	Now() time.Time
}

type systemUnitOverviewClock struct{}

func (systemUnitOverviewClock) Now() time.Time {
	return time.Now()
}

func unitOverviewResponseMetadata(command string, timestamp time.Time) plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin: metadata.PluginName, Version: metadata.Version,
		Command: command, Timestamp: timestamp,
	}
}
