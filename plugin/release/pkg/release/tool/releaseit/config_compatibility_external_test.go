package releaseit_test

import releaseit "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/releaseit"

var (
	_ releaseit.Config        = releaseit.Config{}
	_ releaseit.GithubRelease = releaseit.GithubRelease{}
	_ releaseit.GitConfig     = releaseit.GitConfig{}
	_ releaseit.HooksConfig   = releaseit.HooksConfig{}

	_ func() (*releaseit.Config, error)       = releaseit.LoadConfig
	_ func(string) (*releaseit.Config, error) = releaseit.LoadConfigAt
	_ func(*releaseit.Config) error           = releaseit.SaveConfig
	_ func(string, *releaseit.Config) error   = releaseit.SaveConfigAt
	_ func(string) (*releaseit.Config, error) = releaseit.InitDefaultConfig
)
