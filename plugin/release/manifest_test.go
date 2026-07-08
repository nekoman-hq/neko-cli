package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestManifestExposesResumeCommand(t *testing.T) {
	data, err := os.ReadFile("manifest.json")
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		Commands []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Flags       []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"flags"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	for _, command := range manifest.Commands {
		if command.Name != "resume" {
			continue
		}
		if command.Description == "" {
			t.Fatal("resume description is missing")
		}
		flags := map[string]string{}
		for _, flag := range command.Flags {
			flags[flag.Name] = flag.Type
		}
		if flags["unit"] != "string" || flags["dry-run"] != "bool" {
			t.Fatalf("resume flags missing or invalid: %#v", flags)
		}
		return
	}
	t.Fatal("resume command missing from manifest")
}
