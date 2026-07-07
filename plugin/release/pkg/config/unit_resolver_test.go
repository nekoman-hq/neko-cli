package config

import (
	"strings"
	"testing"
)

func TestResolveReleaseUnitV1(t *testing.T) {
	repo := NormalizeV1Repository("/repo", &V1ReleaseConfig{
		ProjectType:   V1ProjectTypeBackend,
		ReleaseSystem: V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	})

	unit, err := ResolveReleaseUnit(repo, "", UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		t.Fatalf("resolve default: %v", err)
	}
	if unit.ID != "default" {
		t.Fatalf("expected default, got %s", unit.ID)
	}

	unit, err = ResolveReleaseUnit(repo, "default", UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		t.Fatalf("resolve explicit default: %v", err)
	}
	if unit.ID != "default" {
		t.Fatalf("expected default, got %s", unit.ID)
	}

	_, err = ResolveReleaseUnit(repo, "api", UnitResolutionOptions{})
	if err == nil || !strings.Contains(err.Error(), "unknown release unit") {
		t.Fatalf("expected unknown unit error, got %v", err)
	}
}

func TestResolveReleaseUnitV2SingleAndMulti(t *testing.T) {
	single := &ReleaseRepository{
		SourceFormat: SourceFormatV2,
		Units:        []ReleaseUnit{{ID: "api"}},
	}
	unit, err := ResolveReleaseUnit(single, "", UnitResolutionOptions{RequireExplicitForMulti: true})
	if err != nil {
		t.Fatalf("resolve single implicit: %v", err)
	}
	if unit.ID != "api" {
		t.Fatalf("expected api, got %s", unit.ID)
	}

	multi := &ReleaseRepository{
		SourceFormat: SourceFormatV2,
		Units:        []ReleaseUnit{{ID: "web"}, {ID: "api"}},
	}
	_, err = ResolveReleaseUnit(multi, "", UnitResolutionOptions{RequireExplicitForMulti: true})
	if err == nil || !strings.Contains(err.Error(), "api, web") {
		t.Fatalf("expected multi-unit error with ids, got %v", err)
	}

	_, err = ResolveReleaseUnit(multi, "mobile", UnitResolutionOptions{})
	if err == nil || !strings.Contains(err.Error(), "mobile") || !strings.Contains(err.Error(), "api, web") {
		t.Fatalf("expected unknown unit error with ids, got %v", err)
	}
}
