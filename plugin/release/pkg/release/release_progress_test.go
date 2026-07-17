package release

import (
	"reflect"
	"strings"
	"testing"
)

func TestReleaseProgressNoopSuppressesEvents(t *testing.T) {
	releaseProgressOrNoop(nil).ReportReleaseProgress(ReleaseProgressEvent{
		Kind:   ReleaseProgressWorkflowDispatching,
		UnitID: "api",
		Tag:    "api/v1.2.3",
	})
}

func TestReleaseProgressRecorderPreservesEventOrder(t *testing.T) {
	recorder := &recordingReleaseProgress{}
	recorder.ReportReleaseProgress(ReleaseProgressEvent{Kind: ReleaseProgressReleaseStarted, ReleaseType: "patch"})
	recorder.ReportReleaseProgress(ReleaseProgressEvent{Kind: ReleaseProgressTokenPreflightResolving})
	recorder.ReportReleaseProgress(ReleaseProgressEvent{Kind: ReleaseProgressTokenPreflightAvailable})

	want := []ReleaseProgressEventKind{
		ReleaseProgressReleaseStarted,
		ReleaseProgressTokenPreflightResolving,
		ReleaseProgressTokenPreflightAvailable,
	}
	if !reflect.DeepEqual(recorder.kinds(), want) {
		t.Fatalf("recorded progress = %#v, want %#v", recorder.kinds(), want)
	}
}

func TestReleaseProgressEventCarriesOnlySafeTypedData(t *testing.T) {
	eventType := reflect.TypeOf(ReleaseProgressEvent{})
	for index := 0; index < eventType.NumField(); index++ {
		field := eventType.Field(index)
		lowerName := strings.ToLower(field.Name)
		for _, forbidden := range []string{"token", "authorization", "header", "environment", "responsebody", "commandoutput"} {
			if strings.Contains(lowerName, forbidden) {
				t.Fatalf("ReleaseProgressEvent contains secret-bearing field %s", field.Name)
			}
		}
		switch field.Type.Kind() {
		case reflect.Map, reflect.Interface, reflect.Func, reflect.Chan:
			t.Fatalf("ReleaseProgressEvent field %s uses unsupported kind %s", field.Name, field.Type.Kind())
		}
	}
}

type recordingReleaseProgress struct {
	events []ReleaseProgressEvent
}

func (recorder *recordingReleaseProgress) ReportReleaseProgress(event ReleaseProgressEvent) {
	recorder.events = append(recorder.events, event)
}

func (recorder *recordingReleaseProgress) kinds() []ReleaseProgressEventKind {
	kinds := make([]ReleaseProgressEventKind, 0, len(recorder.events))
	for _, event := range recorder.events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}
