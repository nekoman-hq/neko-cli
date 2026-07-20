package releaseit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultConfigSaveAndLoadAtExplicitRoot(t *testing.T) {
	root := t.TempDir()
	config, err := InitDefaultConfig("example")
	if err != nil {
		t.Fatalf("InitDefaultConfig: %v", err)
	}
	if err = SaveConfigAt(root, config); err != nil {
		t.Fatalf("SaveConfigAt: %v", err)
	}
	loaded, err := LoadConfigAt(root)
	if err != nil {
		t.Fatalf("LoadConfigAt: %v", err)
	}
	if !reflect.DeepEqual(loaded, config) {
		t.Fatalf("loaded config = %#v, want %#v", loaded, config)
	}
	content, err := os.ReadFile(filepath.Join(root, ".release-it.json"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	for _, value := range []string{
		`"$schema": "https://unpkg.com/release-it/schema/release-it.json"`,
		`"releaseName": "example@${version}"`,
		`"after:bump": "npx auto-changelog -p"`,
	} {
		if !strings.Contains(string(content), value) {
			t.Errorf("saved config omits %s:\n%s", value, content)
		}
	}
}
