package release

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestReleaseCommandHandlerParsesInvokesOnceAndMapsOutcome(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 9, 30, 0, 0, time.UTC)
	starter := &recordingReleaseCommandStarter{
		outcome: &LegacyReleasePreview{
			ReleaseType:    Minor,
			CurrentVersion: "1.2.3",
			NextVersion:    "1.3.0",
			ReleaseSystem:  "goreleaser",
		},
	}
	handler := releaseCommandHandler{starter: starter, clock: fixedReleaseClock{timestamp}, releaseType: Minor}

	resp, err := handler.Handle(context.Background(), plugin.Request{Flags: map[string]any{"unit": "api", "dry-run": true}})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if starter.calls != 1 || starter.request != (ReleaseCommandRequest{ReleaseType: Minor, UnitID: "api", DryRun: true}) {
		t.Fatalf("starter calls=%d request=%#v", starter.calls, starter.request)
	}
	if resp.Status != "success" || resp.Metadata.Command != "minor" || !resp.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("unexpected mapped response: %#v", resp)
	}
}

func TestReleaseCommandHandlerMapsTypedFailureWithoutGoError(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 10, 0, 0, 0, time.UTC)
	starter := &recordingReleaseCommandStarter{
		failure: &CommandFailure{
			Code:    "CONFIG_NOT_FOUND",
			Message: "missing release config",
			Details: map[string]any{"hint": "initialize release config"},
		},
	}
	handler := releaseCommandHandler{starter: starter, clock: fixedReleaseClock{timestamp}, releaseType: Patch}

	resp, err := handler.Handle(context.Background(), plugin.Request{})

	if err != nil {
		t.Fatalf("Handle returned a Go error: %v", err)
	}
	if starter.calls != 1 || resp.Status != "error" || resp.Error.Code != "CONFIG_NOT_FOUND" {
		t.Fatalf("unexpected failure mapping: calls=%d response=%#v", starter.calls, resp)
	}
	if resp.Error.Details["hint"] != "initialize release config" || !resp.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("failure details or timestamp changed: %#v", resp)
	}
}

func TestReleaseCommandHandlerReturnsFatalPreflightToProcessBoundary(t *testing.T) {
	starter := &recordingReleaseCommandStarter{
		failure: &CommandFailure{
			Code:     "UNCOMMITTED_CHANGES",
			Message:  "working tree is dirty",
			Boundary: CommandFailureFatal,
		},
	}
	handler := releaseCommandHandler{starter: starter, clock: fixedReleaseClock{}, releaseType: Patch}

	resp, err := handler.Handle(context.Background(), plugin.Request{})

	var fatal *FatalCommandError
	if resp != nil || !errors.As(err, &fatal) {
		t.Fatalf("response=%#v error=%v", resp, err)
	}
	if fatal.Code() != "UNCOMMITTED_CHANGES" || fatal.Error() != "working tree is dirty" {
		t.Fatalf("fatal error = code %q message %q", fatal.Code(), fatal.Error())
	}
}

func TestResumeCommandHandlerParsesInvokesOnceAndMapsOutcome(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 11, 0, 0, 0, time.UTC)
	resumer := &recordingReleaseResumer{
		outcome: &ResumeAssessment{
			UnitID:               "api",
			NextVersion:          "1.2.4",
			Tag:                  "api/v1.2.4",
			ExecutionJournalPath: ".git/neko/release/executions/example.json",
			State:                ReleaseExecutionPrepared,
			RecoveryStatus:       ReleaseExecutionRecoveryNotStarted,
			KnownFilePaths:       []string{".neko/release.state.json"},
			Guidance:             "Inspect before continuing.",
		},
	}
	handler := resumeCommandHandler{resumer: resumer, clock: fixedReleaseClock{timestamp}}

	resp, err := handler.Handle(context.Background(), plugin.Request{Flags: map[string]any{"unit": "api", "dry-run": true}})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if resumer.calls != 1 || resumer.request != (ResumeCommandRequest{UnitID: "api", DryRun: true}) {
		t.Fatalf("resumer calls=%d request=%#v", resumer.calls, resumer.request)
	}
	if resp.Status != "success" || resp.Metadata.Command != "resume" || !resp.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("unexpected mapped response: %#v", resp)
	}
}

func TestReleasePlanCommandHandlerParsesInvokesOnceAndMapsOutcome(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC)
	inspector := &recordingReleasePlanInspector{
		inspection: &ReleasePlanInspection{
			Source:          "v2",
			Unit:            ReleasePlanInspectionUnit{ID: "api"},
			CurrentVersion:  "1.2.3",
			RequestedChange: Minor,
			NextVersion:     "1.3.0",
			Tag:             "api/v1.3.0",
			Executor:        "goreleaser",
			Delivery:        "github-actions",
			Readiness:       LocalPlanReady,
		},
	}
	handler := releasePlanCommandHandler{inspector: inspector, clock: fixedReleaseClock{timestamp}}

	resp, err := handler.Handle(context.Background(), plugin.Request{Flags: map[string]any{"change": "minor", "unit": "api"}})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if inspector.calls != 1 || inspector.request != (ReleasePlanInspectionRequest{ReleaseType: Minor, UnitID: "api"}) {
		t.Fatalf("inspector calls=%d request=%#v", inspector.calls, inspector.request)
	}
	if resp.Status != "success" || resp.Metadata.Command != "plan" || !resp.Metadata.Timestamp.Equal(timestamp) {
		t.Fatalf("unexpected mapped response: %#v", resp)
	}
}

func TestReleasePlanCommandHandlerMapsParseFailureWithoutUseCase(t *testing.T) {
	timestamp := time.Date(2026, time.July, 14, 12, 30, 0, 0, time.UTC)
	inspector := &recordingReleasePlanInspector{}
	handler := releasePlanCommandHandler{inspector: inspector, clock: fixedReleaseClock{timestamp}}

	resp, err := handler.Handle(context.Background(), plugin.Request{})

	if err != nil {
		t.Fatalf("Handle returned a Go error: %v", err)
	}
	if inspector.calls != 0 || resp.Status != "error" || resp.Error.Code != "INVALID_RELEASE_CHANGE" {
		t.Fatalf("unexpected parse failure mapping: calls=%d response=%#v", inspector.calls, resp)
	}
}

type fixedReleaseClock struct {
	timestamp time.Time
}

func (clock fixedReleaseClock) Now() time.Time {
	return clock.timestamp
}

type recordingReleaseCommandStarter struct {
	outcome ReleaseCommandOutcome
	failure *CommandFailure
	request ReleaseCommandRequest
	calls   int
}

func (starter *recordingReleaseCommandStarter) Start(_ context.Context, request ReleaseCommandRequest) (ReleaseCommandOutcome, *CommandFailure) {
	starter.calls++
	starter.request = request
	return starter.outcome, starter.failure
}

type recordingReleaseResumer struct {
	outcome ResumeCommandOutcome
	failure *CommandFailure
	request ResumeCommandRequest
	calls   int
}

func (resumer *recordingReleaseResumer) Resume(_ context.Context, request ResumeCommandRequest) (ResumeCommandOutcome, *CommandFailure) {
	resumer.calls++
	resumer.request = request
	return resumer.outcome, resumer.failure
}

type recordingReleasePlanInspector struct {
	inspection *ReleasePlanInspection
	failure    *CommandFailure
	request    ReleasePlanInspectionRequest
	calls      int
}

func (inspector *recordingReleasePlanInspector) Inspect(_ context.Context, request ReleasePlanInspectionRequest) (*ReleasePlanInspection, *CommandFailure) {
	inspector.calls++
	inspector.request = request
	return inspector.inspection, inspector.failure
}
