package release

import (
	"strings"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestUnitOverviewCanonicalMetadataOwners(t *testing.T) {
	version, err := releaseconfig.CanonicalReleaseVersion("1.2.3")
	if err != nil || version != "1.2.3" {
		t.Fatalf("canonical version = %q, %v", version, err)
	}
	tagSpec, err := releaseconfig.NewTagSpec("plugin-release/v")
	if err != nil {
		t.Fatalf("new tag spec: %v", err)
	}
	if got := tagSpec.Format("<version>"); got != "plugin-release/v<version>" {
		t.Fatalf("tag shape = %q", got)
	}
	if got := tagSpec.Format(version); got != "plugin-release/v1.2.3" {
		t.Fatalf("configured tag = %q", got)
	}
}

func TestUnitOverviewCanonicalV2StructurePolicies(t *testing.T) {
	valid := releaseconfig.V2Unit{
		ID:        "api",
		Paths:     []string{"apps/api/**"},
		TagPrefix: "api/v",
		Executor: releaseconfig.V2Executor{
			Type:     releaseconfig.ExecutorGoReleaser,
			Delivery: releaseconfig.DeliveryGitHubActions,
			Workflow: ".github/workflows/release-api.yml",
		},
	}

	tests := []struct {
		name     string
		mutate   func(*releaseconfig.V2Unit)
		contains string
	}{
		{name: "tag prefix", mutate: func(unit *releaseconfig.V2Unit) { unit.TagPrefix = "../unsafe" }, contains: "tagPrefix"},
		{name: "executor", mutate: func(unit *releaseconfig.V2Unit) { unit.Executor.Type = "unknown" }, contains: "executor"},
		{name: "delivery", mutate: func(unit *releaseconfig.V2Unit) { unit.Executor.Delivery = releaseconfig.DeliveryLocal }, contains: "delivery"},
		{name: "workflow", mutate: func(unit *releaseconfig.V2Unit) { unit.Executor.Workflow = "workflows/release.yml" }, contains: "workflow"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			unit := valid
			test.mutate(&unit)
			err := releaseconfig.ValidateV2ReleaseConfigStructure(&releaseconfig.V2ReleaseConfig{
				SchemaVersion: 2,
				Units:         []releaseconfig.V2Unit{unit},
			})
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("validation error = %v, want %q", err, test.contains)
			}
		})
	}

	state := &releaseconfig.V2ReleaseState{
		SchemaVersion: 2,
		Units:         map[string]releaseconfig.V2UnitState{"other": {Version: "1.2.3"}},
	}
	err := releaseconfig.ValidateV2ConfigStateStructure(&releaseconfig.V2ReleaseConfig{
		SchemaVersion: 2,
		Units:         []releaseconfig.V2Unit{valid},
	}, state)
	if err == nil || !releaseconfig.IsV2ConfigStateAlignmentError(err) {
		t.Fatalf("alignment error = %v", err)
	}
}

func TestUnitOverviewCanonicalTagNamespaceRejectsOverlap(t *testing.T) {
	unit := func(id, prefix string) releaseconfig.V2Unit {
		return releaseconfig.V2Unit{
			ID:        id,
			Paths:     []string{id + "/**"},
			TagPrefix: prefix,
			Executor: releaseconfig.V2Executor{
				Type:     releaseconfig.ExecutorGoReleaser,
				Delivery: releaseconfig.DeliveryGitHubActions,
				Workflow: ".github/workflows/release.yml",
			},
		}
	}
	err := releaseconfig.ValidateV2ReleaseConfigStructure(&releaseconfig.V2ReleaseConfig{
		SchemaVersion: 2,
		Units: []releaseconfig.V2Unit{
			unit("api", "service/v"),
			unit("worker", "service/v2"),
		},
	})
	if err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("tag namespace error = %v", err)
	}
}
