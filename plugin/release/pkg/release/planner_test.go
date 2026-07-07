package release

import (
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestPlanUnitVersionBump(t *testing.T) {
	unit := releaseconfig.ReleaseUnit{
		ID:               "api",
		Version:          "0.1.0",
		TagPrefix:        "api/v",
		ExecutorType:     "goreleaser",
		Delivery:         "local",
		WorkingDirectory: "api",
	}

	tests := []struct {
		releaseType Type
		next        string
		tag         string
	}{
		{Patch, "0.1.1", "api/v0.1.1"},
		{Minor, "0.2.0", "api/v0.2.0"},
		{Major, "1.0.0", "api/v1.0.0"},
	}

	for _, tt := range tests {
		plan, err := PlanUnitVersionBump(unit, tt.releaseType)
		if err != nil {
			t.Fatalf("PlanUnitVersionBump(%s): %v", tt.releaseType, err)
		}
		if plan.NextVersion != tt.next || plan.Tag != tt.tag || plan.UnitID != "api" {
			t.Fatalf("unexpected plan: %#v", plan)
		}
	}
}
