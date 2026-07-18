package config

import (
	"strings"
	"testing"
)

func TestV2LocalDeliveryConfigurationIsRejected(t *testing.T) {
	for _, executor := range []ExecutorType{ExecutorGoReleaser, ExecutorJReleaser, ExecutorReleaseIt} {
		t.Run(string(executor), func(t *testing.T) {
			cfg := &V2ReleaseConfig{SchemaVersion: 2, Units: []V2Unit{{
				ID:        "api",
				Paths:     []string{"api/**"},
				TagPrefix: "api/v",
				Executor:  V2Executor{Type: executor, Delivery: DeliveryLocal},
			}}}

			err := ValidateV2ReleaseConfigStructure(cfg)
			if err == nil || !strings.Contains(err.Error(), "local delivery is not supported") {
				t.Fatalf("expected local delivery rejection for %s, got %v", executor, err)
			}
		})
	}
}

func TestV2MissingDeliveryNoLongerDefaultsToLocal(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, root+"/api")
	writeV2Files(t, root, validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {"type": "jreleaser"}
}`), validV2State(`"api": {"version": "1.0.0"}`))

	_, err := LoadV2Repository(root)
	if err == nil || !strings.Contains(err.Error(), "executor.delivery is required") {
		t.Fatalf("expected missing delivery rejection, got %v", err)
	}
}
