package init

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestCommandParsersProduceDistinctTypedRequestsWithoutRawMaps(t *testing.T) {
	flags := validPluginInitFlags()
	flags["force"] = true
	initRequest := parseInitCommandRequest(flags)
	addRequest := parseUnitAddCommandRequest(flags)

	if initRequest.Unit.UnitID != "plugin-release" || !initRequest.Force {
		t.Fatalf("unexpected init request: %#v", initRequest)
	}
	if addRequest.Unit.Plugin.Name != "release" || !addRequest.ForcePresent {
		t.Fatalf("unexpected unit-add request: %#v", addRequest)
	}
	if reflect.TypeOf(initRequest) == reflect.TypeOf(addRequest) {
		t.Fatal("init and unit-add requests must be distinct types")
	}
	for _, requestType := range []reflect.Type{reflect.TypeOf(initRequest), reflect.TypeOf(addRequest), reflect.TypeOf(initRequest.Unit)} {
		for index := 0; index < requestType.NumField(); index++ {
			if requestType.Field(index).Type.Kind() == reflect.Map {
				t.Fatalf("typed request %s retains raw map field %s", requestType, requestType.Field(index).Name)
			}
		}
	}
}

func TestConstructV2UnitHasSeparateReleaseAndPluginResults(t *testing.T) {
	t.Run("defaults and wrong raw types", func(t *testing.T) {
		flags := map[string]any{
			"unit":         42,
			"display-name": false,
			"executor":     "goreleaser",
			"delivery":     "github-actions",
			"workflow":     ".github/workflows/release-cli.yml",
			"force":        "true",
		}
		request := parseInitCommandRequest(flags)
		constructed, err := constructV2Unit(request.Unit)
		if err != nil {
			t.Fatalf("constructV2Unit: %v", err)
		}
		if request.Force || constructed.Config.UnitID != "cli" || constructed.Config.DisplayName != "cli" || constructed.Config.Version != "0.1.0" {
			t.Fatalf("defaults or raw-type compatibility changed: request=%#v result=%#v", request, constructed.Config)
		}
	})

	t.Run("release", func(t *testing.T) {
		flags := validInitFlags()
		constructed, err := constructV2Unit(parseV2UnitRequest(flags))
		if err != nil {
			t.Fatalf("constructV2Unit: %v", err)
		}
		if constructed.Unit.Kind != "" || constructed.Unit.Plugin != nil || constructed.Unit.Executor.Workflow != ".github/workflows/release-cli.yml" {
			t.Fatalf("unexpected release unit: %#v", constructed.Unit)
		}
		if constructed.Config.Kind != defaultKind || constructed.State.Version != "0.1.0" {
			t.Fatalf("unexpected release construction result: %#v", constructed)
		}
	})

	t.Run("plugin", func(t *testing.T) {
		constructed, err := constructV2Unit(parseV2UnitRequest(validPluginInitFlags()))
		if err != nil {
			t.Fatalf("constructV2Unit: %v", err)
		}
		if constructed.Unit.Kind != config.UnitKindPlugin || constructed.Unit.Plugin == nil {
			t.Fatalf("unexpected plugin unit: %#v", constructed.Unit)
		}
		if constructed.Unit.Plugin.Name != "release" || constructed.Config.Kind != pluginKind {
			t.Fatalf("unexpected plugin construction result: %#v", constructed)
		}
	})
}

func TestV2FilePoliciesArePureAndPreserveCompatibilityCodes(t *testing.T) {
	tests := []struct { //nolint:govet // Policy inputs and expected codes stay adjacent.
		name     string
		presence v2RepositoryPresence
		force    bool
		initCode string
		addCode  string
	}{
		{name: "empty", presence: v2RepositoryPresence{}, addCode: "V2_CONFIG_MISSING"},
		{name: "v1", presence: v2RepositoryPresence{LegacyConfig: true}, initCode: "V1_CONFIG_EXISTS", addCode: "V1_CONFIG_EXISTS"},
		{name: "config", presence: v2RepositoryPresence{Config: true}, initCode: "CONFIG_EXISTS", addCode: "PARTIAL_V2_CONFIG"},
		{name: "state", presence: v2RepositoryPresence{State: true}, initCode: "CONFIG_EXISTS", addCode: "PARTIAL_V2_CONFIG"},
		{name: "pair", presence: v2RepositoryPresence{Config: true, State: true}, initCode: "CONFIG_EXISTS"},
		{name: "pair forced", presence: v2RepositoryPresence{Config: true, State: true}, force: true},
		{name: "conflict", presence: v2RepositoryPresence{LegacyConfig: true, Config: true, State: true}, force: true, initCode: "CONFIG_CONFLICT", addCode: "CONFIG_CONFLICT"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := test.presence
			assertPolicyCode(t, evaluateV2InitializationPolicy(test.presence, test.force), test.initCode)
			assertPolicyCode(t, evaluateV2UnitAdditionPolicy(test.presence), test.addCode)
			if test.presence != before {
				t.Fatalf("policy mutated presence: before=%#v after=%#v", before, test.presence)
			}
		})
	}
}

func TestV2PairConstructionAndAppendDoNotMutateInputs(t *testing.T) {
	constructed, err := constructV2Unit(parseV2UnitRequest(validPluginInitFlags()))
	if err != nil {
		t.Fatalf("construct plugin: %v", err)
	}
	created := newV2ReleasePair(constructed)
	constructed.Unit.Paths[0] = "mutated/**"
	constructed.Unit.Plugin.Name = "mutated"
	if created.Config.Units[0].Paths[0] != "plugin/release/**" || created.Config.Units[0].Plugin.Name != "release" {
		t.Fatalf("new pair aliases constructed unit: %#v", created.Config.Units[0])
	}

	current := testV2ReleasePair("cli", "0.1.0")
	current.Config.Units[0].Plugin = &config.V2Plugin{Name: "existing"}
	currentConfigBytes, err := config.CanonicalV2Config(current.Config)
	if err != nil {
		t.Fatalf("canonical current config: %v", err)
	}
	currentStateBytes, err := config.CanonicalV2State(current.State)
	if err != nil {
		t.Fatalf("canonical current state: %v", err)
	}
	updated := appendV2ReleaseUnit(current, createdUnitForAppend())
	updated.Config.Units[0].Paths[0] = "changed/**"
	updated.Config.Units[0].Plugin.Name = "changed"
	updated.State.Units["cli"] = config.V2UnitState{Version: "9.9.9"}

	afterConfigBytes, err := config.CanonicalV2Config(current.Config)
	if err != nil {
		t.Fatalf("canonical current config after append: %v", err)
	}
	afterStateBytes, err := config.CanonicalV2State(current.State)
	if err != nil {
		t.Fatalf("canonical current state after append: %v", err)
	}
	if !reflect.DeepEqual(afterConfigBytes, currentConfigBytes) || !reflect.DeepEqual(afterStateBytes, currentStateBytes) {
		t.Fatalf("append mutated current pair:\n%s\n%s", afterConfigBytes, afterStateBytes)
	}
}

func TestInitializeV2RepositoryUseCaseValidatesBeforePersistingCompletePair(t *testing.T) {
	operations := []string{}
	presence := &recordingPresenceReader{operations: &operations}
	validator := &recordingPairValidator{operations: &operations}
	writer := &recordingPairWriter{operations: &operations}
	useCase := initializeV2RepositoryUseCase{presenceReader: presence, validator: validator, writer: writer}
	result, failure := useCase.Execute(parseInitCommandRequest(validInitFlags()))
	if failure != nil {
		t.Fatalf("Execute failure: %#v", failure)
	}
	if result.Unit.UnitID != "cli" || !reflect.DeepEqual(operations, []string{"presence", "validate", "persist"}) {
		t.Fatalf("unexpected result/order: %#v %v", result, operations)
	}
	if len(writer.persisted.Config.Units) != 1 || len(writer.persisted.State.Units) != 1 || writer.persisted.State.Units["cli"].Version != "0.1.0" {
		t.Fatalf("incomplete persisted pair: %#v", writer.persisted)
	}

	operations = []string{}
	presence = &recordingPresenceReader{operations: &operations}
	validator = &recordingPairValidator{operations: &operations, err: errors.New("invalid pair")}
	writer = &recordingPairWriter{operations: &operations}
	useCase = initializeV2RepositoryUseCase{presenceReader: presence, validator: validator, writer: writer}
	_, failure = useCase.Execute(parseInitCommandRequest(validInitFlags()))
	if failure == nil || failure.code != "VALIDATION_ERROR" || !reflect.DeepEqual(operations, []string{"presence", "validate"}) {
		t.Fatalf("validation boundary changed: failure=%#v operations=%v", failure, operations)
	}
}

func TestInitializeV2RepositoryUseCaseSurfacesPairPersistenceFailure(t *testing.T) {
	operations := []string{}
	presence := &recordingPresenceReader{operations: &operations}
	validator := &recordingPairValidator{operations: &operations}
	writer := &recordingPairWriter{
		operations: &operations,
		err:        errors.New("rollback failed; manual recovery required"),
	}
	useCase := initializeV2RepositoryUseCase{presenceReader: presence, validator: validator, writer: writer}

	_, failure := useCase.Execute(parseInitCommandRequest(validInitFlags()))
	if failure == nil || failure.code != "SAVE_ERROR" || !strings.Contains(failure.message, "manual recovery required") {
		t.Fatalf("unexpected persistence failure: %#v", failure)
	}
	if !reflect.DeepEqual(operations, []string{"presence", "validate", "persist"}) {
		t.Fatalf("persistence failure operations = %v", operations)
	}
}

func TestAddV2ReleaseUnitUseCaseLoadsOnceAndPersistsAppendedCopy(t *testing.T) {
	current := testV2ReleasePair("cli", "0.1.0")
	operations := []string{}
	presence := &recordingPresenceReader{
		operations: &operations,
		presence:   v2RepositoryPresence{Config: true, State: true},
	}
	loader := &recordingPairLoader{operations: &operations, loaded: current}
	validator := &recordingPairValidator{operations: &operations}
	writer := &recordingPairWriter{operations: &operations}
	useCase := addV2ReleaseUnitUseCase{presenceReader: presence, loader: loader, validator: validator, writer: writer}

	result, failure := useCase.Execute(parseUnitAddCommandRequest(validUnitAddFlags()))
	if failure != nil {
		t.Fatalf("Execute failure: %#v", failure)
	}
	if result.Unit.UnitID != "api" || loader.loadCount != 1 {
		t.Fatalf("unexpected result/load count: %#v %d", result, loader.loadCount)
	}
	if !reflect.DeepEqual(operations, []string{"presence", "load", "validate", "persist"}) {
		t.Fatalf("operations = %v", operations)
	}
	if len(writer.persisted.Config.Units) != 2 || writer.persisted.Config.Units[0].ID != "cli" || writer.persisted.Config.Units[1].ID != "api" {
		t.Fatalf("unit order changed: %#v", writer.persisted.Config.Units)
	}
	if _, exists := current.State.Units["api"]; exists || len(current.Config.Units) != 1 {
		t.Fatalf("use case mutated loaded pair: %#v", current)
	}
}

func TestAddV2ReleaseUnitUseCaseStopsAtLoadAndDuplicateFailures(t *testing.T) {
	t.Run("load", func(t *testing.T) {
		operations := []string{}
		presence := &recordingPresenceReader{
			operations: &operations,
			presence:   v2RepositoryPresence{Config: true, State: true},
		}
		loader := &recordingPairLoader{
			operations: &operations,
			err:        &v2PairLoadError{err: errors.New("broken state"), part: "state"},
		}
		validator := &recordingPairValidator{operations: &operations}
		writer := &recordingPairWriter{operations: &operations}
		useCase := addV2ReleaseUnitUseCase{presenceReader: presence, loader: loader, validator: validator, writer: writer}
		_, failure := useCase.Execute(parseUnitAddCommandRequest(validUnitAddFlags()))
		if failure == nil || failure.code != "LOAD_ERROR" || !strings.Contains(failure.message, "release state") {
			t.Fatalf("unexpected load failure: %#v", failure)
		}
		if !reflect.DeepEqual(operations, []string{"presence", "load"}) {
			t.Fatalf("operations after load failure = %v", operations)
		}
	})

	t.Run("duplicate state", func(t *testing.T) {
		current := testV2ReleasePair("cli", "0.1.0")
		current.State.Units["api"] = config.V2UnitState{Version: "1.0.0"}
		operations := []string{}
		presence := &recordingPresenceReader{
			operations: &operations,
			presence:   v2RepositoryPresence{Config: true, State: true},
		}
		loader := &recordingPairLoader{operations: &operations, loaded: current}
		validator := &recordingPairValidator{operations: &operations}
		writer := &recordingPairWriter{operations: &operations}
		useCase := addV2ReleaseUnitUseCase{presenceReader: presence, loader: loader, validator: validator, writer: writer}
		_, failure := useCase.Execute(parseUnitAddCommandRequest(validUnitAddFlags()))
		if failure == nil || failure.code != "DUPLICATE_UNIT" {
			t.Fatalf("unexpected duplicate failure: %#v", failure)
		}
		if !reflect.DeepEqual(operations, []string{"presence", "load"}) {
			t.Fatalf("operations after duplicate = %v", operations)
		}
	})
}

func TestInitAndUnitAddHandlersContainOnlyCommandBoundaryDependencies(t *testing.T) {
	source, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("read handler.go: %v", err)
	}
	for _, forbidden := range []string{"CanonicalV2", "LoadV2", "ValidateV2", "AtomicWrite", "os.", "req.Flags["} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("handler.go contains workflow dependency %q", forbidden)
		}
	}
}

func createdUnitForAppend() constructedV2Unit {
	return constructedV2Unit{
		Config: v2InitConfig{UnitID: "api", Version: "1.2.3", Kind: defaultKind},
		Unit: config.V2Unit{
			ID:               "api",
			Paths:            []string{"api/**"},
			WorkingDirectory: ".",
			TagPrefix:        "api/v",
			Executor: config.V2Executor{
				Type:     config.ExecutorGoReleaser,
				Delivery: config.DeliveryGitHubActions,
				Workflow: ".github/workflows/release-api.yml",
			},
		},
		State: config.V2UnitState{Version: "1.2.3"},
	}
}

func assertPolicyCode(t *testing.T, failure *v2PresencePolicyFailure, want string) {
	t.Helper()
	if want == "" {
		if failure != nil {
			t.Fatalf("unexpected policy failure: %v", failure)
		}
		return
	}
	if failure == nil || failure.code != want {
		t.Fatalf("policy code = %#v, want %s", failure, want)
	}
}

type recordingPresenceReader struct {
	operations *[]string
	presence   v2RepositoryPresence
}

func (reader *recordingPresenceReader) Presence() v2RepositoryPresence {
	*reader.operations = append(*reader.operations, "presence")
	return reader.presence
}

type recordingPairLoader struct {
	operations *[]string
	loaded     v2ReleasePair
	err        error
	loadCount  int
}

func (loader *recordingPairLoader) LoadPair() (v2ReleasePair, error) {
	*loader.operations = append(*loader.operations, "load")
	loader.loadCount++
	return loader.loaded, loader.err
}

type recordingPairValidator struct {
	operations *[]string
	validated  v2ReleasePair
	err        error
}

func (validator *recordingPairValidator) ValidatePair(pair v2ReleasePair) error {
	*validator.operations = append(*validator.operations, "validate")
	validator.validated = pair
	return validator.err
}

type recordingPairWriter struct {
	operations *[]string
	persisted  v2ReleasePair
	err        error
}

func (writer *recordingPairWriter) PersistPair(pair v2ReleasePair) error {
	*writer.operations = append(*writer.operations, "persist")
	writer.persisted = pair
	return writer.err
}
