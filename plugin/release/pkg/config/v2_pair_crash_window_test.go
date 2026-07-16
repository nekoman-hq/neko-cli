package config

import (
	"os"
	"strings"
	"testing"
)

func TestV2ReleasePairCrashWindowWithoutEvidenceLeavesMixedPairInvalid(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root+"/"+V2Directory, 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}

	oldPair := testV2ReleasePair("api", "1.2.3")
	oldConfig, err := CanonicalV2Config(oldPair.Config)
	if err != nil {
		t.Fatalf("old config: %v", err)
	}
	oldState, err := CanonicalV2State(oldPair.State)
	if err != nil {
		t.Fatalf("old state: %v", err)
	}
	if err := os.WriteFile(V2ConfigPath(root), oldConfig, 0600); err != nil {
		t.Fatalf("write old config: %v", err)
	}
	if err := os.WriteFile(V2StatePath(root), oldState, 0640); err != nil {
		t.Fatalf("write old state: %v", err)
	}

	nextPair := testV2ReleasePair("web", "2.0.0")
	nextConfig, err := CanonicalV2Config(nextPair.Config)
	if err != nil {
		t.Fatalf("next config: %v", err)
	}
	if err := os.Remove(V2ConfigPath(root)); err != nil {
		t.Fatalf("remove old config before simulated rename: %v", err)
	}
	if err := os.WriteFile(V2ConfigPath(root), nextConfig, 0644); err != nil {
		t.Fatalf("simulate config-only crash: %v", err)
	}

	_, err = LoadV2Repository(root)
	if err == nil || !strings.Contains(err.Error(), "missing unit") {
		t.Fatalf("mixed pair error = %v, want strict config/state mismatch", err)
	}
	assertPairCrashFileMode(t, V2ConfigPath(root), 0644)
	assertPairCrashFileMode(t, V2StatePath(root), 0640)
}

func TestV2ReleasePairCrashWindowWithoutEvidenceLeavesIncompleteNewPairInvalid(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(root+"/"+V2Directory, 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}

	pair := testV2ReleasePair("api", "1.2.3")
	configData, err := CanonicalV2Config(pair.Config)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	if err := os.WriteFile(V2ConfigPath(root), configData, 0644); err != nil {
		t.Fatalf("simulate config-only create crash: %v", err)
	}

	_, err = LoadV2Repository(root)
	if err == nil || !strings.Contains(err.Error(), "open "+V2StatePath(root)) {
		t.Fatalf("incomplete new pair error = %v, want missing state", err)
	}
	assertPairCrashFileMode(t, V2ConfigPath(root), 0644)
	if _, statErr := os.Stat(V2StatePath(root)); !os.IsNotExist(statErr) {
		t.Fatalf("state unexpectedly exists after simulated crash: %v", statErr)
	}
}

func assertPairCrashFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %04o, want %04o", path, got, want)
	}
}
