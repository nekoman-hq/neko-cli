package config

import (
	"path/filepath"
	"testing"
)

func TestV2LocalDeliveryCurrentConfigurationContract(t *testing.T) {
	for _, executor := range []ExecutorType{ExecutorGoReleaser, ExecutorJReleaser, ExecutorReleaseIt} {
		t.Run(string(executor), func(t *testing.T) {
			cfg := &V2ReleaseConfig{SchemaVersion: 2, Units: []V2Unit{{
				ID:        "api",
				Paths:     []string{"api/**"},
				TagPrefix: "api/v",
				Executor:  V2Executor{Type: executor, Delivery: DeliveryLocal},
			}}}

			if err := ValidateV2ReleaseConfigStructure(cfg); err != nil {
				t.Fatalf("local delivery currently validates for %s: %v", executor, err)
			}
		})
	}
}

func TestV2LocalDeliveryCurrentRepositoryContractDoesNotRequireWorkflow(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "api"))
	writeV2Files(t, root, validV2Config(`{
  "id": "api",
  "paths": ["api/**"],
  "workingDirectory": "api",
  "tagPrefix": "api/v",
  "executor": {"type": "jreleaser", "delivery": "local"}
}`), validV2State(`"api": {"version": "1.0.0"}`))

	repository, err := LoadV2Repository(root)
	if err != nil {
		t.Fatalf("LoadV2Repository: %v", err)
	}
	if repository.Units[0].Delivery != string(DeliveryLocal) || repository.Units[0].Workflow != "" {
		t.Fatalf("unexpected current local delivery normalization: %#v", repository.Units[0])
	}
}
