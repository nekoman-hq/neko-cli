//nolint:staticcheck // Tests intentionally construct the retained V1 source model.
package release

import (
	"context"
	"errors"
	"testing"

	releaseconfig "github.com/nekoman-hq/neko-cli/plugin/release/pkg/config"
)

type fixedReleaseRepositoryReader struct {
	repository *releaseconfig.ReleaseRepository
	err        error
	calls      int
}

func (reader *fixedReleaseRepositoryReader) Load(string) (*releaseconfig.ReleaseRepository, error) {
	reader.calls++
	return reader.repository, reader.err
}

type recordingV1ReleaseApplication struct {
	calls int
}

func (application *recordingV1ReleaseApplication) Start(
	context.Context,
	*releaseconfig.ReleaseRepository,
	ReleaseCommandRequest,
) (ReleaseCommandOutcome, *CommandFailure) {
	application.calls++
	return &V1ReleasePreview{}, nil
}

type recordingV2ReleaseApplication struct {
	calls int
}

func (application *recordingV2ReleaseApplication) Start(
	context.Context,
	*releaseconfig.ReleaseRepository,
	ReleaseCommandRequest,
) (ReleaseCommandOutcome, *CommandFailure) {
	application.calls++
	return &V2ReleasePreview{}, nil
}

func TestReleaseSourceSelectionChoosesExactlyOneApplication(t *testing.T) {
	tests := []struct { //nolint:govet // Logical selector cases keep source and expected calls together.
		name        string
		format      releaseconfig.SourceFormat
		wantV1      int
		wantV2      int
		wantFailure string
	}{
		{name: "v1", format: releaseconfig.SourceFormatV1, wantV1: 1},
		{name: "v2", format: releaseconfig.SourceFormatV2, wantV2: 1},
		{name: "unknown", format: releaseconfig.SourceFormat("v3"), wantFailure: "SOURCE_FORMAT_UNSUPPORTED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repositories := &fixedReleaseRepositoryReader{repository: &releaseconfig.ReleaseRepository{SourceFormat: tt.format}}
			v1 := &recordingV1ReleaseApplication{}
			v2 := &recordingV2ReleaseApplication{}
			operation := releaseStartOperation{repositories: repositories, v1: v1, v2: v2}

			_, failure := operation.Start(context.Background(), ReleaseCommandRequest{ReleaseType: Patch})

			if repositories.calls != 1 || v1.calls != tt.wantV1 || v2.calls != tt.wantV2 {
				t.Fatalf("calls: repositories=%d v1=%d v2=%d", repositories.calls, v1.calls, v2.calls)
			}
			if tt.wantFailure == "" && failure != nil {
				t.Fatalf("unexpected failure: %#v", failure)
			}
			if tt.wantFailure != "" && (failure == nil || failure.Code != tt.wantFailure) {
				t.Fatalf("failure = %#v, want %s", failure, tt.wantFailure)
			}
		})
	}
}

func TestReleaseSourceSelectionStopsAfterRepositoryFailure(t *testing.T) {
	repositories := &fixedReleaseRepositoryReader{err: errors.New("load failed")}
	v1 := &recordingV1ReleaseApplication{}
	v2 := &recordingV2ReleaseApplication{}
	operation := releaseStartOperation{repositories: repositories, v1: v1, v2: v2}

	_, failure := operation.Start(context.Background(), ReleaseCommandRequest{ReleaseType: Patch})

	if failure == nil || failure.Code != "CONFIG_NOT_FOUND" || v1.calls != 0 || v2.calls != 0 {
		t.Fatalf("failure=%#v v1=%d v2=%d", failure, v1.calls, v2.calls)
	}
}

func TestReleaseApplicationPathSelectorIsTypedAndPure(t *testing.T) {
	if path, err := selectReleaseApplicationPath(releaseconfig.SourceFormatV1); err != nil || path != releaseconfig.SourceFormatV1 {
		t.Fatalf("V1 selection = %v, %v", path, err)
	}
	if path, err := selectReleaseApplicationPath(releaseconfig.SourceFormatV2); err != nil || path != releaseconfig.SourceFormatV2 {
		t.Fatalf("V2 selection = %v, %v", path, err)
	}
	if _, err := selectReleaseApplicationPath(releaseconfig.SourceFormat("future")); err == nil {
		t.Fatal("unknown source format was accepted")
	}
}
