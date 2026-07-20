package init

const (
	defaultUnitID           = "cli"
	defaultInitialVersion   = "0.1.0"
	defaultTagPrefix        = "v"
	defaultWorkingDirectory = "."
	defaultPaths            = "**"
	defaultKind             = "release"
	pluginKind              = "plugin"
	legacyV1ConfigFileName  = ".release.neko.json"
)

var pluginInitFlagNames = []string{
	"plugin-name",
	"plugin-manifest",
	"plugin-asset-prefix",
	"plugin-binary-name",
}
