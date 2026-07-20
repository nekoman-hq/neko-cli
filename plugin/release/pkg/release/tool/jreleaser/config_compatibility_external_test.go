package jreleaser_test

import jreleaser "github.com/nekoman-hq/neko-cli/plugin/release/pkg/release/tool/jreleaser"

var (
	_ jreleaser.Config                        = jreleaser.Config{}
	_ jreleaser.Project                       = jreleaser.Project{}
	_ jreleaser.ProjectLinks                  = jreleaser.ProjectLinks{}
	_ jreleaser.ProjectLanguages              = jreleaser.ProjectLanguages{}
	_ jreleaser.JavaLanguage                  = jreleaser.JavaLanguage{}
	_ jreleaser.Release                       = jreleaser.Release{}
	_ jreleaser.GithubRelease                 = jreleaser.GithubRelease{}
	_ jreleaser.Changelog                     = jreleaser.Changelog{}
	_ jreleaser.Contributors                  = jreleaser.Contributors{}
	_ jreleaser.ChangelogAppend               = jreleaser.ChangelogAppend{}
	_ jreleaser.Labeler                       = jreleaser.Labeler{}
	_ jreleaser.Category                      = jreleaser.Category{}
	_ func() (*jreleaser.Config, error)       = jreleaser.LoadConfig
	_ func(string) (*jreleaser.Config, error) = jreleaser.LoadConfigAt
	_ func(*jreleaser.Config) error           = jreleaser.SaveConfig
	_ func(string, *jreleaser.Config) error   = jreleaser.SaveConfigAt
)
