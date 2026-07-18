package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2PairRecoveryEvidenceValidationFailsClosed(t *testing.T) {
	root := t.TempDir()
	store := newV2PairRecoveryStore(root)
	evidence := testV2PairRecoveryEvidence(t, root)

	t.Run("unknown schema", func(t *testing.T) {
		invalid := evidence
		invalid.SchemaVersion = 99
		assertV2PairEvidenceInvalid(t, root, invalid, "unsupported V2 pair recovery schema version")
	})

	t.Run("unknown replacement", func(t *testing.T) {
		invalid := evidence
		invalid.ConfigReplacement = v2PairReplacementEvidence("advanced")
		assertV2PairEvidenceInvalid(t, root, invalid, "unknown config replacement")
	})

	t.Run("unknown restoration", func(t *testing.T) {
		invalid := evidence
		invalid.Restoration = v2PairRestorationEvidence("advanced")
		assertV2PairEvidenceInvalid(t, root, invalid, "unknown restoration")
	})

	t.Run("invalid hash", func(t *testing.T) {
		invalid := evidence
		invalid.IntendedState.SHA256 = strings.Repeat("0", 64)
		assertV2PairEvidenceInvalid(t, root, invalid, "sha256 mismatch")
	})

	t.Run("invalid mode", func(t *testing.T) {
		invalid := evidence
		invalid.PriorConfig.Mode = 01000
		assertV2PairEvidenceInvalid(t, root, invalid, "invalid mode")
	})

	t.Run("store load wraps manual recovery", func(t *testing.T) {
		invalid := evidence
		invalid.StateReplacement = v2PairReplacementEvidence("future")
		if err := os.MkdirAll(filepath.Dir(store.path), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := AtomicWriteJSON(store.path, &invalid, 0644); err != nil {
			t.Fatalf("write invalid evidence: %v", err)
		}
		_, err := store.LoadUnresolved()
		var recoveryError *V2PairRecoveryError
		if !errors.As(err, &recoveryError) || !recoveryError.ManualRecoveryRequired() {
			t.Fatalf("load error = %T %v, want manual V2 pair recovery error", err, err)
		}
	})
}

func TestSelectV2PairRecoveryOperation(t *testing.T) {
	root := t.TempDir()
	evidence := testV2PairRecoveryEvidence(t, root)

	tests := []struct {
		name        string
		observation v2PairRecoveryObservation
		want        v2PairRecoveryDecisionKind
	}{
		{
			name: "complete intended pair",
			observation: v2PairRecoveryObservation{
				config:            observedV2PairFile(evidence.IntendedConfig),
				state:             observedV2PairFile(evidence.IntendedState),
				intendedPairValid: true,
			},
			want: v2PairRecoveryAlreadyComplete,
		},
		{
			name: "partial intended pair restores original",
			observation: v2PairRecoveryObservation{
				config: observedV2PairFile(evidence.IntendedConfig),
				state:  observedV2PairFile(evidence.PriorState),
			},
			want: v2PairRecoveryRestoreOriginal,
		},
		{
			name: "unchanged original closes through restoration path",
			observation: v2PairRecoveryObservation{
				config: observedV2PairFile(evidence.PriorConfig),
				state:  observedV2PairFile(evidence.PriorState),
			},
			want: v2PairRecoveryRestoreOriginal,
		},
		{
			name: "conflicting config refuses",
			observation: v2PairRecoveryObservation{
				config: v2PairObservedFile{exists: true, sha: sha256HexBytes([]byte("external"))},
				state:  observedV2PairFile(evidence.PriorState),
			},
			want: v2PairRecoveryManual,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decision := selectV2PairRecoveryOperation(evidence, test.observation)
			if decision.kind != test.want {
				t.Fatalf("decision = %#v, want %d", decision, test.want)
			}
		})
	}
}

func TestValidateV2PairRecoveryReadinessIsReadOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, V2Directory), 0o755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	if err := ValidateV2PairRecoveryReadiness(root); err != nil {
		t.Fatalf("readiness without evidence: %v", err)
	}

	evidence := testV2PairRecoveryEvidence(t, root)
	store := newV2PairRecoveryStore(root)
	if err := store.CreatePairRecoveryEvidence(evidence); err != nil {
		t.Fatalf("create evidence: %v", err)
	}
	before, err := os.ReadFile(V2PairRecoveryPath(root))
	if err != nil {
		t.Fatalf("read evidence before: %v", err)
	}
	if readinessErr := ValidateV2PairRecoveryReadiness(root); readinessErr == nil {
		t.Fatal("readiness must fail while unresolved evidence exists")
	}
	after, err := os.ReadFile(V2PairRecoveryPath(root))
	if err != nil {
		t.Fatalf("read evidence after: %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatal("readiness check changed pair recovery evidence")
	}
}

func TestV2PairRecoveryRestoresOriginalPairInNextProcess(t *testing.T) {
	root := t.TempDir()
	writeDefaultWorkflow(t, root)
	writeV2PairForRecoveryTest(t, root, testV2ReleasePair("api", "1.2.3"), 0600, 0640)
	oldConfig := mustReadV2PairFile(t, V2ConfigPath(root))
	oldState := mustReadV2PairFile(t, V2StatePath(root))
	nextPair := testV2ReleasePair("web", "2.0.0")
	nextConfig, nextState := canonicalPairBytesForRecoveryTest(t, nextPair)

	evidence := newV2PairRecoveryEvidence(
		root,
		v2FileSnapshot{data: oldConfig, mode: 0600, exists: true},
		v2FileSnapshot{data: oldState, mode: 0640, exists: true},
		nextConfig,
		nextState,
	)
	evidence.ConfigReplacement = v2PairReplacementConfirmed
	store := newV2PairRecoveryStore(root)
	if err := os.MkdirAll(filepath.Dir(store.path), 0755); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}
	if err := store.CreatePairRecoveryEvidence(evidence); err != nil {
		t.Fatalf("create evidence: %v", err)
	}
	if err := os.Remove(V2ConfigPath(root)); err != nil {
		t.Fatalf("remove old config: %v", err)
	}
	if err := os.WriteFile(V2ConfigPath(root), nextConfig, 0644); err != nil {
		t.Fatalf("simulate config replacement: %v", err)
	}

	nextProcess := NewV2ReleasePairPersister(root)
	if err := nextProcess.recoverUnresolvedPair(); err != nil {
		t.Fatalf("recover unresolved pair: %v", err)
	}
	assertV2PairFileBytesAndMode(t, V2ConfigPath(root), oldConfig, 0600)
	assertV2PairFileBytesAndMode(t, V2StatePath(root), oldState, 0640)
	if _, err := os.Stat(V2PairRecoveryPath(root)); !os.IsNotExist(err) {
		t.Fatalf("recovery evidence was not closed: %v", err)
	}
}

func TestV2PairRecoveryClosesCompletedIntendedPairInNextProcess(t *testing.T) {
	root := t.TempDir()
	writeDefaultWorkflow(t, root)
	nextPair := testV2ReleasePair("api", "1.2.3")
	nextConfig, nextState := canonicalPairBytesForRecoveryTest(t, nextPair)
	evidence := newV2PairRecoveryEvidence(root, v2FileSnapshot{}, v2FileSnapshot{}, nextConfig, nextState)
	evidence.ConfigReplacement = v2PairReplacementConfirmed
	evidence.StateReplacement = v2PairReplacementConfirmed
	store := newV2PairRecoveryStore(root)
	if err := os.MkdirAll(filepath.Dir(store.path), 0755); err != nil {
		t.Fatalf("mkdir evidence dir: %v", err)
	}
	if err := store.CreatePairRecoveryEvidence(evidence); err != nil {
		t.Fatalf("create evidence: %v", err)
	}
	if err := os.WriteFile(V2ConfigPath(root), nextConfig, 0644); err != nil {
		t.Fatalf("write intended config: %v", err)
	}
	if err := os.WriteFile(V2StatePath(root), nextState, 0644); err != nil {
		t.Fatalf("write intended state: %v", err)
	}

	nextProcess := NewV2ReleasePairPersister(root)
	if err := nextProcess.recoverUnresolvedPair(); err != nil {
		t.Fatalf("recover completed pair: %v", err)
	}
	assertV2PairFileBytesAndMode(t, V2ConfigPath(root), nextConfig, 0644)
	assertV2PairFileBytesAndMode(t, V2StatePath(root), nextState, 0644)
	if _, err := os.Stat(V2PairRecoveryPath(root)); !os.IsNotExist(err) {
		t.Fatalf("completed recovery evidence was not closed: %v", err)
	}
}

func testV2PairRecoveryEvidence(t *testing.T, root string) v2PairRecoveryEvidence {
	t.Helper()
	oldPair := testV2ReleasePair("api", "1.2.3")
	oldConfig, oldState := canonicalPairBytesForRecoveryTest(t, oldPair)
	nextPair := testV2ReleasePair("web", "2.0.0")
	nextConfig, nextState := canonicalPairBytesForRecoveryTest(t, nextPair)
	return newV2PairRecoveryEvidence(
		root,
		v2FileSnapshot{data: oldConfig, mode: 0600, exists: true},
		v2FileSnapshot{data: oldState, mode: 0640, exists: true},
		nextConfig,
		nextState,
	)
}

func assertV2PairEvidenceInvalid(t *testing.T, root string, evidence v2PairRecoveryEvidence, want string) {
	t.Helper()
	err := evidence.validate(root)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("validation error = %v, want %q", err, want)
	}
}

func observedV2PairFile(file v2PairRecoveryFile) v2PairObservedFile {
	return v2PairObservedFile{
		exists: file.Exists,
		mode:   file.Mode,
		sha:    file.SHA256,
	}
}

func canonicalPairBytesForRecoveryTest(t *testing.T, pair V2ReleasePair) ([]byte, []byte) {
	t.Helper()
	configData, err := CanonicalV2Config(pair.Config)
	if err != nil {
		t.Fatalf("canonical config: %v", err)
	}
	stateData, err := CanonicalV2State(pair.State)
	if err != nil {
		t.Fatalf("canonical state: %v", err)
	}
	return configData, stateData
}

func writeV2PairForRecoveryTest(t *testing.T, root string, pair V2ReleasePair, configMode, stateMode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, V2Directory), 0755); err != nil {
		t.Fatalf("mkdir .neko: %v", err)
	}
	configData, stateData := canonicalPairBytesForRecoveryTest(t, pair)
	if err := os.WriteFile(V2ConfigPath(root), configData, configMode); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(V2StatePath(root), stateData, stateMode); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func mustReadV2PairFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func assertV2PairFileBytesAndMode(t *testing.T, path string, want []byte, wantMode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != string(want) {
		t.Fatalf("%s bytes = %q, want %q", path, data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != wantMode {
		t.Fatalf("%s mode = %04o, want %04o", path, got, wantMode)
	}
}
