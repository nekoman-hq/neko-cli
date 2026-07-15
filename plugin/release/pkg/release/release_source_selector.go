package release

import (
	"fmt"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

// selectReleaseApplicationPath is the single pure boundary that selects the
// V1 compatibility application or the V2 application from loaded metadata.
func selectReleaseApplicationPath(sourceFormat releaseconfig.SourceFormat) (releaseconfig.SourceFormat, error) {
	switch sourceFormat {
	case releaseconfig.SourceFormatV1:
		return releaseconfig.SourceFormatV1, nil
	case releaseconfig.SourceFormatV2:
		return releaseconfig.SourceFormatV2, nil
	default:
		return releaseconfig.SourceFormat(""), fmt.Errorf("unsupported release source format %q", sourceFormat)
	}
}
