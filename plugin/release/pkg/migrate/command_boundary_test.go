package migrate

import (
	"errors"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestParseMigrationCommandRequestKeepsRawFlagsAtBoundary(t *testing.T) {
	tests := []struct { //nolint:govet // Parser cases read clearly in name, input, expected order.
		name  string
		flags map[string]any
		want  bool
	}{
		{name: "true", flags: map[string]any{"dry-run": true}, want: true},
		{name: "false", flags: map[string]any{"dry-run": false}, want: false},
		{name: "missing", flags: nil, want: false},
		{name: "wrong type", flags: map[string]any{"dry-run": "true"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := parseMigrationCommandRequest(test.flags, "/work")
			if request.startDirectory != "/work" || request.preview != test.want {
				t.Fatalf("request = %#v, want preview %t", request, test.want)
			}
		})
	}
}

func TestMigrationCommandHandlerInvokesUseCaseOnceAndMapsAtFixedTime(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	plan := completedMigrationPlan("/repo", migrationPaths("/repo"))
	useCase := &fakeMigrationCommandUseCase{result: migrationCommandResult{plan: plan}}
	handler := migrationCommandHandler{useCase: useCase, now: func() time.Time { return fixedTime }}

	resp, err := handler.Handle(plugin.Request{
		Flags:   map[string]any{"dry-run": true},
		Context: plugin.Context{WorkingDir: "/work"},
	})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if useCase.calls != 1 || useCase.received != (migrationCommandRequest{startDirectory: "/work", preview: true}) {
		t.Fatalf("use case calls = %d request = %#v", useCase.calls, useCase.received)
	}
	if resp.Status != "success" || resp.Metadata.Timestamp != fixedTime || resp.Metadata.Command != "migrate" {
		t.Fatalf("response mapping changed: %#v", resp)
	}
}

func TestMigrationCommandHandlerMapsTypedFailureWithNilGoError(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	useCase := &fakeMigrationCommandUseCase{failure: &migrationFailure{cause: errors.New("failed")}}
	handler := migrationCommandHandler{useCase: useCase, now: func() time.Time { return fixedTime }}

	resp, err := handler.Handle(plugin.Request{})
	if err != nil {
		t.Fatalf("handler Go error = %v, want nil", err)
	}
	if resp.Status != "error" || resp.Error == nil || resp.Error.Code != "MIGRATION_FAILED" || resp.Error.Message != "failed" || resp.Metadata.Timestamp != fixedTime {
		t.Fatalf("failure response changed: %#v", resp)
	}
}

type fakeMigrationCommandUseCase struct { //nolint:govet // Fake fields follow request, result, and observation order.
	received migrationCommandRequest
	result   migrationCommandResult
	failure  *migrationFailure
	calls    int
}

func (useCase *fakeMigrationCommandUseCase) Migrate(request migrationCommandRequest) (migrationCommandResult, *migrationFailure) {
	useCase.calls++
	useCase.received = request
	return useCase.result, useCase.failure
}
