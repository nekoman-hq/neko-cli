package init

import (
	"encoding/json"
	"os"
	"testing"
)

func TestBuildConfigFromFlagsUsesVersionFlag(t *testing.T) {
	cfg, err := buildConfigFromFlags(map[string]any{
		"project-type":   "backend",
		"release-system": "goreleaser",
		"version":        "2.3.4",
	})
	if err != nil {
		t.Fatalf("buildConfigFromFlags: %v", err)
	}

	if cfg.Version != "2.3.4" {
		t.Fatalf("expected version 2.3.4, got %s", cfg.Version)
	}
}

func TestBuildConfigFromFlagsUsesMetadataOnlyAsFallback(t *testing.T) {
	cfg, err := buildConfigFromFlags(map[string]any{
		"project-type":   "backend",
		"release-system": "goreleaser",
		"version":        "2.3.4",
		"metadata":       "9.9.9",
	})
	if err != nil {
		t.Fatalf("buildConfigFromFlags: %v", err)
	}
	if cfg.Version != "2.3.4" {
		t.Fatalf("expected canonical version to win, got %s", cfg.Version)
	}

	cfg, err = buildConfigFromFlags(map[string]any{
		"project-type":   "backend",
		"release-system": "goreleaser",
		"metadata":       "1.2.3",
	})
	if err != nil {
		t.Fatalf("buildConfigFromFlags fallback: %v", err)
	}
	if cfg.Version != "1.2.3" {
		t.Fatalf("expected metadata fallback version 1.2.3, got %s", cfg.Version)
	}
}

func TestManifestExposesVersionFlag(t *testing.T) {
	data, err := os.ReadFile("../../manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Commands []struct {
			Name  string `json:"name"`
			Flags []struct {
				Name string `json:"name"`
			} `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	for _, command := range manifest.Commands {
		if command.Name != "init" {
			continue
		}
		for _, flag := range command.Flags {
			if flag.Name == "version" {
				return
			}
		}
		t.Fatal("init command does not expose version flag")
	}

	t.Fatal("init command not found")
}
