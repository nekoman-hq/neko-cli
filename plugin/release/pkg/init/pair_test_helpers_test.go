package init

import "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"

func testV2ReleasePair(unitID, version string) v2ReleasePair {
	return v2ReleasePair{
		Config: config.V2ReleaseConfig{
			SchemaVersion: 2,
			Units: []config.V2Unit{{
				ID:               unitID,
				Paths:            []string{"**"},
				WorkingDirectory: ".",
				TagPrefix:        unitID + "/v",
				Executor: config.V2Executor{
					Type:     config.ExecutorGoReleaser,
					Delivery: config.DeliveryGitHubActions,
					Workflow: ".github/workflows/release-cli.yml",
				},
			}},
		},
		State: config.V2ReleaseState{
			SchemaVersion: 2,
			Units: map[string]config.V2UnitState{
				unitID: {Version: version},
			},
		},
	}
}
