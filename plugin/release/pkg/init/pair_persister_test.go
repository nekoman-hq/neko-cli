package init

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestV2ReleasePairPersisterSuccessPreparesBothBeforeReplace(t *testing.T) {
	pair := testV2ReleasePair("api", "1.2.3")
	disk := newFakeV2PairDisk()
	persister := v2ReleasePairPersister{disk: disk}

	if err := persister.Persist(pair); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	wantOperations := []string{
		"mkdir", "capture-config", "capture-state", "create-config-temp", "write-config-temp",
		"create-state-temp", "write-state-temp",
		"replace-config", "replace-state", "discard-state", "discard-config",
	}
	if !reflect.DeepEqual(disk.operations, wantOperations) {
		t.Fatalf("operations = %v, want %v", disk.operations, wantOperations)
	}
	wantConfig, err := config.CanonicalV2Config(pair.Config)
	if err != nil {
		t.Fatalf("canonical config: %v", err)
	}
	wantState, err := config.CanonicalV2State(pair.State)
	if err != nil {
		t.Fatalf("canonical state: %v", err)
	}
	if !reflect.DeepEqual(disk.configData, wantConfig) || !reflect.DeepEqual(disk.stateData, wantState) {
		t.Fatalf("prepared bytes differ: config=%s state=%s", disk.configData, disk.stateData)
	}
	if disk.configMode != 0644 || disk.stateMode != 0644 {
		t.Fatalf("prepared modes = %04o/%04o, want 0644/0644", disk.configMode, disk.stateMode)
	}
}

func TestV2ReleasePairPersisterPreparationFailuresDoNotReplace(t *testing.T) {
	pair := testV2ReleasePair("api", "1.2.3")

	t.Run("directory", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.createErr = errors.New("mkdir denied")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "create .neko directory", disk.operations, []string{"mkdir"})
	})

	t.Run("config snapshot", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.captureConfigErr = errors.New("config unreadable")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "capture existing V2 config", disk.operations, []string{"mkdir", "capture-config"})
	})

	t.Run("state snapshot", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.captureStateErr = errors.New("state unreadable")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "capture existing V2 state", disk.operations, []string{"mkdir", "capture-config", "capture-state"})
	})

	t.Run("config temp create", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.createConfigTempErr = errors.New("config temp create failed")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "create V2 config temp file", disk.operations, []string{
			"mkdir", "capture-config", "capture-state", "create-config-temp",
		})
	})

	t.Run("config temp write", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.writeConfigErr = errors.New("config temp write failed")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "write V2 config temp file", disk.operations, []string{
			"mkdir", "capture-config", "capture-state", "create-config-temp", "write-config-temp", "discard-config",
		})
	})

	t.Run("state temp create", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.createStateTempErr = errors.New("state temp create failed")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "create V2 state temp file", disk.operations, []string{
			"mkdir", "capture-config", "capture-state", "create-config-temp", "write-config-temp",
			"create-state-temp", "discard-config",
		})
	})

	t.Run("state temp write", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.writeStateErr = errors.New("state temp write failed")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "write V2 state temp file", disk.operations, []string{
			"mkdir", "capture-config", "capture-state", "create-config-temp", "write-config-temp",
			"create-state-temp", "write-state-temp", "discard-state", "discard-config",
		})
	})
}

func TestV2ReleasePairPersisterReplaceFailuresRestoreBothSnapshots(t *testing.T) {
	pair := testV2ReleasePair("api", "1.2.3")
	configSnapshot := v2FileSnapshot{data: []byte("old config"), mode: 0600, exists: true}
	stateSnapshot := v2FileSnapshot{data: []byte("old state"), mode: 0640, exists: true}

	t.Run("first replace", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.configSnapshot = configSnapshot
		disk.stateSnapshot = stateSnapshot
		disk.replaceConfigErr = errors.New("config rename failed")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "previous config/state pair restored", disk.operations, []string{
			"mkdir", "capture-config", "capture-state", "create-config-temp", "write-config-temp",
			"create-state-temp", "write-state-temp",
			"replace-config", "restore-config", "restore-state", "discard-state", "discard-config",
		})
		assertRestoredSnapshots(t, disk, configSnapshot, stateSnapshot)
	})

	t.Run("second replace", func(t *testing.T) {
		disk := newFakeV2PairDisk()
		disk.configSnapshot = configSnapshot
		disk.stateSnapshot = stateSnapshot
		disk.replaceStateErr = errors.New("state rename failed")
		err := (v2ReleasePairPersister{disk: disk}).Persist(pair)
		assertPersistenceErrorAndOperations(t, err, "previous config/state pair restored", disk.operations, []string{
			"mkdir", "capture-config", "capture-state", "create-config-temp", "write-config-temp",
			"create-state-temp", "write-state-temp",
			"replace-config", "replace-state", "restore-config", "restore-state", "discard-state", "discard-config",
		})
		assertRestoredSnapshots(t, disk, configSnapshot, stateSnapshot)
	})
}

func TestV2ReleasePairPersisterRestorationFailureRequiresManualRecovery(t *testing.T) {
	disk := newFakeV2PairDisk()
	disk.replaceStateErr = errors.New("state rename failed")
	disk.restoreConfigErr = errors.New("config restore failed")
	disk.restoreStateErr = errors.New("state restore failed")

	err := (v2ReleasePairPersister{disk: disk}).Persist(testV2ReleasePair("api", "1.2.3"))
	var persistenceError *v2PairPersistenceError
	if !errors.As(err, &persistenceError) {
		t.Fatalf("restoration failure type = %T, want *v2PairPersistenceError", err)
	}
	for _, want := range []string{"rollback failed", "restore V2 config", "restore V2 state", "manual recovery required"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("error %v does not contain %q", err, want)
		}
	}
	if !reflect.DeepEqual(disk.operations[len(disk.operations)-4:], []string{
		"restore-config", "restore-state", "discard-state", "discard-config",
	}) {
		t.Fatalf("both restorations and cleanups were not attempted: %v", disk.operations)
	}
}

func TestV2ReleasePairPersisterRestoresExactFilesOnSecondReplaceFailure(t *testing.T) {
	root := t.TempDir()
	configPath := config.V2ConfigPath(root)
	statePath := config.V2StatePath(root)
	if err := os.MkdirAll(root+"/"+config.V2Directory, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldConfig := []byte("old config bytes\n")
	oldState := []byte("old state bytes\n")
	if err := os.WriteFile(configPath, oldConfig, 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(statePath, oldState, 0640); err != nil {
		t.Fatalf("write state: %v", err)
	}
	disk := &failAfterStateReplaceDisk{osV2PairPersistenceDisk: osV2PairPersistenceDisk{root: root}}

	err := (v2ReleasePairPersister{disk: disk}).Persist(testV2ReleasePair("api", "1.2.3"))
	if err == nil || !strings.Contains(err.Error(), "previous config/state pair restored") {
		t.Fatalf("expected recovered second-replace failure, got %v", err)
	}
	configData, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read restored config: %v", readErr)
	}
	stateData, readErr := os.ReadFile(statePath)
	if readErr != nil {
		t.Fatalf("read restored state: %v", readErr)
	}
	configInfo, statErr := os.Stat(configPath)
	if statErr != nil {
		t.Fatalf("stat restored config: %v", statErr)
	}
	stateInfo, statErr := os.Stat(statePath)
	if statErr != nil {
		t.Fatalf("stat restored state: %v", statErr)
	}
	if !reflect.DeepEqual(configData, oldConfig) || configInfo.Mode().Perm() != 0600 {
		t.Fatalf("config restoration changed bytes/mode: %q %04o", configData, configInfo.Mode().Perm())
	}
	if !reflect.DeepEqual(stateData, oldState) || stateInfo.Mode().Perm() != 0640 {
		t.Fatalf("state restoration changed bytes/mode: %q %04o", stateData, stateInfo.Mode().Perm())
	}
	assertNoPairTemps(t, root+"/"+config.V2Directory)
}

func TestV2ReleasePairPersisterRemovesNewPairOnSecondReplaceFailure(t *testing.T) {
	root := t.TempDir()
	disk := &failAfterStateReplaceDisk{osV2PairPersistenceDisk: osV2PairPersistenceDisk{root: root}}

	err := (v2ReleasePairPersister{disk: disk}).Persist(testV2ReleasePair("api", "1.2.3"))
	if err == nil || !strings.Contains(err.Error(), "previous config/state pair restored") {
		t.Fatalf("expected recovered creation failure, got %v", err)
	}
	for _, path := range []string{config.V2ConfigPath(root), config.V2StatePath(root)} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("new target survived rollback: %s (%v)", path, statErr)
		}
	}
	assertNoPairTemps(t, root+"/"+config.V2Directory)
}

type failAfterStateReplaceDisk struct {
	osV2PairPersistenceDisk
}

func (disk *failAfterStateReplaceDisk) CreateStateTemp() (v2TemporaryFile, error) {
	prepared, err := disk.osV2PairPersistenceDisk.CreateStateTemp()
	if err != nil {
		return nil, err
	}
	return &failAfterReplaceFile{prepared: prepared}, nil
}

type failAfterReplaceFile struct {
	prepared v2TemporaryFile
}

func (file *failAfterReplaceFile) Replace() error {
	if err := file.prepared.Replace(); err != nil {
		return err
	}
	return errors.New("injected failure after state replacement")
}

func (file *failAfterReplaceFile) Write(data []byte, mode os.FileMode) error {
	return file.prepared.Write(data, mode)
}

func (file *failAfterReplaceFile) Discard() {
	file.prepared.Discard()
}

func testV2ReleasePair(unitID, version string) v2ReleasePair {
	return v2ReleasePair{
		Config: config.V2ReleaseConfig{
			SchemaVersion: 2,
			Units: []config.V2Unit{{
				ID:               unitID,
				Paths:            []string{"**"},
				WorkingDirectory: ".",
				TagPrefix:        unitID + "/v",
				Executor: config.V2Executor{
					Type:     config.ExecutorGoReleaser,
					Delivery: config.DeliveryLocal,
				},
			}},
		},
		State: config.V2ReleaseState{
			SchemaVersion: 2,
			Units: map[string]config.V2UnitState{
				unitID: {Version: version},
			},
		},
	}
}

type fakeV2PairDisk struct { //nolint:govet // Failure controls stay grouped by persistence phase.
	operations          []string
	configSnapshot      v2FileSnapshot
	stateSnapshot       v2FileSnapshot
	configRestored      v2FileSnapshot
	stateRestored       v2FileSnapshot
	configData          []byte
	stateData           []byte
	configMode          os.FileMode
	stateMode           os.FileMode
	createErr           error
	captureConfigErr    error
	captureStateErr     error
	createConfigTempErr error
	createStateTempErr  error
	writeConfigErr      error
	writeStateErr       error
	replaceConfigErr    error
	replaceStateErr     error
	restoreConfigErr    error
	restoreStateErr     error
}

func newFakeV2PairDisk() *fakeV2PairDisk {
	return &fakeV2PairDisk{}
}

func (disk *fakeV2PairDisk) CreateDirectory() error {
	disk.operations = append(disk.operations, "mkdir")
	return disk.createErr
}

func (disk *fakeV2PairDisk) CaptureConfig() (v2FileSnapshot, error) {
	disk.operations = append(disk.operations, "capture-config")
	return disk.configSnapshot, disk.captureConfigErr
}

func (disk *fakeV2PairDisk) CaptureState() (v2FileSnapshot, error) {
	disk.operations = append(disk.operations, "capture-state")
	return disk.stateSnapshot, disk.captureStateErr
}

func (disk *fakeV2PairDisk) CreateConfigTemp() (v2TemporaryFile, error) {
	disk.operations = append(disk.operations, "create-config-temp")
	if disk.createConfigTempErr != nil {
		return nil, disk.createConfigTempErr
	}
	return &fakePreparedV2File{
		disk:       disk,
		name:       "config",
		writeErr:   disk.writeConfigErr,
		replaceErr: disk.replaceConfigErr,
	}, nil
}

func (disk *fakeV2PairDisk) CreateStateTemp() (v2TemporaryFile, error) {
	disk.operations = append(disk.operations, "create-state-temp")
	if disk.createStateTempErr != nil {
		return nil, disk.createStateTempErr
	}
	return &fakePreparedV2File{
		disk:       disk,
		name:       "state",
		writeErr:   disk.writeStateErr,
		replaceErr: disk.replaceStateErr,
	}, nil
}

func (disk *fakeV2PairDisk) RestoreConfig(snapshot v2FileSnapshot) error {
	disk.operations = append(disk.operations, "restore-config")
	disk.configRestored = snapshot
	return disk.restoreConfigErr
}

func (disk *fakeV2PairDisk) RestoreState(snapshot v2FileSnapshot) error {
	disk.operations = append(disk.operations, "restore-state")
	disk.stateRestored = snapshot
	return disk.restoreStateErr
}

type fakePreparedV2File struct { //nolint:govet // Focused fake keeps identity before injected failures.
	disk       *fakeV2PairDisk
	name       string
	writeErr   error
	replaceErr error
}

func (prepared *fakePreparedV2File) Write(data []byte, mode os.FileMode) error {
	prepared.disk.operations = append(prepared.disk.operations, "write-"+prepared.name+"-temp")
	if prepared.name == "config" {
		prepared.disk.configData = append([]byte(nil), data...)
		prepared.disk.configMode = mode
	} else {
		prepared.disk.stateData = append([]byte(nil), data...)
		prepared.disk.stateMode = mode
	}
	return prepared.writeErr
}

func (prepared *fakePreparedV2File) Replace() error {
	prepared.disk.operations = append(prepared.disk.operations, "replace-"+prepared.name)
	return prepared.replaceErr
}

func (prepared *fakePreparedV2File) Discard() {
	prepared.disk.operations = append(prepared.disk.operations, "discard-"+prepared.name)
}

func assertPersistenceErrorAndOperations(t *testing.T, err error, text string, got, want []string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want text %q", err, text)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}

func assertRestoredSnapshots(t *testing.T, disk *fakeV2PairDisk, wantConfig, wantState v2FileSnapshot) {
	t.Helper()
	if !reflect.DeepEqual(disk.configRestored, wantConfig) || !reflect.DeepEqual(disk.stateRestored, wantState) {
		t.Fatalf("restored snapshots = %#v/%#v, want %#v/%#v", disk.configRestored, disk.stateRestored, wantConfig, wantState)
	}
}

func assertNoPairTemps(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read pair directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file leaked: %s", entry.Name())
		}
	}
}
