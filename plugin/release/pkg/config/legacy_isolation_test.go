package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLegacyIsolationFileNames(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	configDir := filepath.Dir(currentFile)

	if _, err := os.Stat(filepath.Join(configDir, "config.go")); !os.IsNotExist(err) {
		t.Fatalf("generic config.go must not contain V1-only code; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "release_config.go")); !os.IsNotExist(err) {
		t.Fatalf("generic release_config.go must not contain V1-only code; stat err=%v", err)
	}
	for _, file := range []string{"v1.go", "v1_loader.go", "v2.go"} {
		if _, err := os.Stat(filepath.Join(configDir, file)); err != nil {
			t.Fatalf("expected %s: %v", file, err)
		}
	}
}

func TestExportedV1APIsHaveDeprecatedGoDoc(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	configDir := filepath.Dir(currentFile)

	for _, file := range []string{"v1.go", "v1_loader.go"} {
		data, err := os.ReadFile(filepath.Join(configDir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		if !strings.Contains(string(data), "Deprecated: V1 is supported only as the legacy compatibility format.") {
			t.Fatalf("%s is missing Deprecated GoDoc for exported V1 APIs", file)
		}
	}
}
