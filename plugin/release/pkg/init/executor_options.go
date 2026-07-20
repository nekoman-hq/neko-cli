package init

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/internal/releasetool"
)

func supportedExecutorValues(separator string) string {
	identities := releasetool.Identities()
	values := make([]string, len(identities))
	for index, identity := range identities {
		values[index] = string(identity)
	}
	return strings.Join(values, separator)
}

func supportedExecutorDescription() string {
	identities := releasetool.Identities()
	if len(identities) == 0 {
		return ""
	}
	if len(identities) == 1 {
		return string(identities[0])
	}
	values := make([]string, len(identities)-1)
	for index, identity := range identities[:len(identities)-1] {
		values[index] = string(identity)
	}
	return strings.Join(values, ", ") + ", or " + string(identities[len(identities)-1])
}
