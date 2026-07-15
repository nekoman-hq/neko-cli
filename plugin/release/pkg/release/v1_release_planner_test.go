//nolint:staticcheck // Tests intentionally construct the retained V1 source model.
package release

import (
	"reflect"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestV1ReleasePlannerBuildsDeterministicPlans(t *testing.T) {
	tests := []struct {
		name        string
		releaseType Type
		next        string
	}{
		{name: "patch", releaseType: Patch, next: "1.2.4"},
		{name: "minor", releaseType: Minor, next: "1.3.0"},
		{name: "major", releaseType: Major, next: "2.0.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validV1ReleaseConfig("1.2.3")
			intent := V1ReleaseIntent{
				RepositoryRoot: "/repo",
				Unit:           releaseconfig.ReleaseUnit{ID: "default"},
				Config:         cfg,
				ReleaseType:    tt.releaseType,
			}
			request := V1ReleasePlanningRequest{Intent: intent, LatestTag: "v1.2.3"}

			first, err := PlanV1Release(request)
			if err != nil {
				t.Fatalf("PlanV1Release: %v", err)
			}
			second, err := PlanV1Release(request)
			if err != nil {
				t.Fatalf("second PlanV1Release: %v", err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("nondeterministic plans:\n%#v\n%#v", first, second)
			}
			if first.CurrentVersion != "1.2.3" || first.NextVersion != tt.next || first.Tag != "v"+tt.next {
				t.Fatalf("unexpected version plan: %#v", first)
			}
			if first.CommitMessage != "chore(neko-release): "+tt.next || first.Executor != "goreleaser" {
				t.Fatalf("unexpected executor/commit plan: %#v", first)
			}
			if got := first.MaterializedFiles(); !reflect.DeepEqual(got, []string{releaseconfig.V1FileName}) {
				t.Fatalf("materialized files = %#v", got)
			}
			if cfg.Version != "1.2.3" {
				t.Fatalf("planner mutated V1 config to %q", cfg.Version)
			}
			files := first.MaterializedFiles()
			files[0] = "changed"
			if first.MaterializedFiles()[0] != releaseconfig.V1FileName {
				t.Fatal("planner exposed mutable file ownership")
			}
		})
	}
}

func TestV1ReleasePlannerPreservesLatestTagRules(t *testing.T) {
	intent := V1ReleaseIntent{
		RepositoryRoot: "/repo",
		Unit:           releaseconfig.ReleaseUnit{ID: "default"},
		Config:         validV1ReleaseConfig("1.2.3"),
		ReleaseType:    Patch,
	}
	plan, err := PlanV1Release(V1ReleasePlanningRequest{Intent: intent, LatestTag: "not-semver"})
	if err != nil {
		t.Fatalf("invalid latest tag must fall back to local: %v", err)
	}
	if plan.IgnoredLatestTag != "not-semver" || plan.LatestVersion != "" {
		t.Fatalf("latest tag evidence = %#v", plan)
	}

	intent.Config = validV1ReleaseConfig("1.2.2")
	if _, err := PlanV1Release(V1ReleasePlanningRequest{Intent: intent, LatestTag: "v1.2.3"}); err == nil {
		t.Fatal("local version behind latest tag was accepted")
	}
}
