//nolint:staticcheck // V1 compatibility tests intentionally exercise deprecated V1 models.
package validate

//lint:file-ignore SA1019 V1 validation compatibility intentionally uses deprecated V1 APIs during migration

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

func TestParseValidationQueryRequestOwnsRawFlags(t *testing.T) {
	tests := []struct {
		name  string
		flags map[string]any
		want  validationQueryRequest
	}{
		{name: "missing", flags: nil, want: validationQueryRequest{}},
		{name: "wrong types", flags: map[string]any{"show": "true", "unit": false}, want: validationQueryRequest{}},
		{name: "values", flags: map[string]any{"show": true, "unit": "api"}, want: validationQueryRequest{Show: true, Unit: "api"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseValidationQueryRequest(tt.flags); got != tt.want {
				t.Fatalf("parse = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestValidateCommandHandlerInvokesOneQueryAndMapsResult(t *testing.T) {
	fixed := time.Date(2026, time.July, 15, 9, 30, 0, 0, time.UTC)
	query := &recordingValidationQuerier{result: validationQueryResult{
		SourceFormat: config.SourceFormatV2,
		Show:         true,
		Units: []config.ReleaseUnit{{
			ID:               "api",
			Version:          "1.2.3",
			WorkingDirectory: ".",
			TagPrefix:        "api/v",
			ExecutorType:     "goreleaser",
			Delivery:         "github-actions",
			Workflow:         ".github/workflows/release-api.yml",
			Paths:            []string{"api/**"},
		}},
	}}
	handler := validateCommandHandler{query: query, clock: fixedValidationClock{now: fixed}}

	resp, err := handler.Handle(plugin.Request{Flags: map[string]any{"show": true, "unit": "api"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if query.calls != 1 || query.request != (validationQueryRequest{Show: true, Unit: "api"}) {
		t.Fatalf("query calls/request = %d/%#v", query.calls, query.request)
	}
	if resp.Status != "success" || resp.RendererHint != "table" || !resp.Metadata.Timestamp.Equal(fixed) {
		t.Fatalf("unexpected response: %#v", resp)
	}
	want := []map[string]any{
		{"property": "Schema", "value": "v2"},
		{"property": "Unit api", "value": "version=1.2.3 workingDirectory=. tagPrefix=api/v executor=goreleaser delivery=github-actions workflow=.github/workflows/release-api.yml paths=[api/**]"},
	}
	if got := resp.Data["items"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestValidationQueryUseCaseClassifiesRepositoryFailures(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		hint    string
		present bool
	}{
		{name: "missing", present: false, code: "CONFIG_NOT_FOUND", message: "No release configuration found", hint: missingConfigurationHint},
		{name: "invalid", present: true, code: "CONFIG_INVALID", message: "load failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requirements := &fakeLegacyRequirementsValidator{}
			reader := &fakeValidationRepositoryReader{present: tt.present, err: errors.New("load failed")}
			useCase := newValidationQueryUseCase(reader, requirements)

			_, failure := useCase.Query(validationQueryRequest{})
			if failure == nil || failure.Code != tt.code || failure.Message != tt.message || failure.Hint != tt.hint {
				t.Fatalf("failure = %#v", failure)
			}
			if reader.calls != 1 || reader.root != "." || requirements.calls != 0 {
				t.Fatalf("unexpected dependency calls: reader=%#v requirements=%#v", reader, requirements)
			}
		})
	}
}

func TestValidationQueryUseCaseReadsV2WithoutLegacyRequirementsOrMutation(t *testing.T) {
	repository := &config.ReleaseRepository{
		SourceFormat: config.SourceFormatV2,
		Units: []config.ReleaseUnit{
			{ID: "api", Paths: []string{"api/**"}, Version: "1.2.3"},
			{ID: "web", Paths: []string{"web/**"}, Version: "2.0.0"},
		},
	}
	requirements := &fakeLegacyRequirementsValidator{err: errors.New("must not run")}
	useCase := newValidationQueryUseCase(&fakeValidationRepositoryReader{repository: repository, present: true}, requirements)

	result, failure := useCase.Query(validationQueryRequest{Show: true, Unit: "api"})
	if failure != nil {
		t.Fatalf("Query failure: %#v", failure)
	}
	if requirements.calls != 0 || len(result.Units) != 1 || result.Units[0].ID != "api" ||
		result.SelectedUnit != "api" || result.ConfiguredUnitCount != 2 {
		t.Fatalf("result/dependencies = %#v/%#v", result, requirements)
	}
	result.Units[0].Paths[0] = "mutated"
	if repository.Units[0].Paths[0] != "api/**" {
		t.Fatalf("query result aliases repository paths: %#v", repository.Units)
	}
}

func TestValidationQueryUseCasePreservesV1ValidationOrder(t *testing.T) {
	t.Run("requirements failure follows model validation", func(t *testing.T) {
		legacy := validLegacyValidationConfig()
		requirements := &fakeLegacyRequirementsValidator{err: errors.New("token missing")}
		useCase := newValidationQueryUseCase(
			&fakeValidationRepositoryReader{repository: config.NormalizeV1Repository(".", legacy), present: true},
			requirements,
		)

		_, failure := useCase.Query(validationQueryRequest{})
		if failure == nil || failure.Code != "VALIDATION_FAILED" || failure.Message != "token missing" {
			t.Fatalf("failure = %#v", failure)
		}
		if requirements.calls != 1 || requirements.cfg != legacy {
			t.Fatalf("requirements calls/config = %d/%p, want 1/%p", requirements.calls, requirements.cfg, legacy)
		}
	})

	t.Run("invalid model stops before requirements", func(t *testing.T) {
		legacy := validLegacyValidationConfig()
		legacy.Version = ""
		requirements := &fakeLegacyRequirementsValidator{}
		useCase := newValidationQueryUseCase(
			&fakeValidationRepositoryReader{repository: config.NormalizeV1Repository(".", legacy), present: true},
			requirements,
		)

		_, failure := useCase.Query(validationQueryRequest{})
		if failure == nil || failure.Code != "VALIDATION_FAILED" {
			t.Fatalf("failure = %#v", failure)
		}
		if requirements.calls != 0 {
			t.Fatalf("requirements called after model failure: %#v", requirements)
		}
	})
}

//nolint:govet // Test fake field order groups returned values before captured calls.
type recordingValidationQuerier struct {
	result  validationQueryResult
	failure *validationQueryFailure
	request validationQueryRequest
	calls   int
}

func (query *recordingValidationQuerier) Query(request validationQueryRequest) (validationQueryResult, *validationQueryFailure) {
	query.calls++
	query.request = request
	return query.result, query.failure
}

type fixedValidationClock struct{ now time.Time }

func (clock fixedValidationClock) Now() time.Time { return clock.now }

type fakeValidationRepositoryReader struct {
	repository *config.ReleaseRepository
	err        error
	root       string
	present    bool
	calls      int
}

func (reader *fakeValidationRepositoryReader) Read(root string) (*config.ReleaseRepository, bool, error) {
	reader.calls++
	reader.root = root
	return reader.repository, reader.present, reader.err
}

type fakeLegacyRequirementsValidator struct {
	cfg   *config.V1ReleaseConfig
	err   error
	calls int
}

func (validator *fakeLegacyRequirementsValidator) Validate(cfg *config.V1ReleaseConfig) error {
	validator.calls++
	validator.cfg = cfg
	return validator.err
}

func validLegacyValidationConfig() *config.V1ReleaseConfig {
	return &config.V1ReleaseConfig{
		ProjectName:   "neko-cli",
		ProjectOwner:  "nekoman-hq",
		ProjectType:   config.V1ProjectTypeBackend,
		ReleaseSystem: config.V1ReleaseTypeGoReleaser,
		Version:       "1.2.3",
	}
}
