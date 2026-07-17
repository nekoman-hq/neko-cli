package pluginindex

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestParsePluginIndexCommandRequestProducesTypedModesAndDefaults(t *testing.T) {
	tests := []struct {
		name    string
		flags   map[string]any
		want    pluginIndexCommandRequest
		wantErr bool
	}{
		{
			name: "default render",
			want: pluginIndexCommandRequest{Repository: DefaultRepository, Mode: pluginIndexRenderMode, Pretty: true},
		},
		{
			name:  "wrong types keep defaults",
			flags: map[string]any{"output": true, "check": "true", "pretty": "false", "repository": false},
			want:  pluginIndexCommandRequest{Repository: DefaultRepository, Mode: pluginIndexRenderMode, Pretty: true},
		},
		{
			name:  "check",
			flags: map[string]any{"check": true, "pretty": false, "repository": "example/project"},
			want:  pluginIndexCommandRequest{Repository: "example/project", Mode: pluginIndexCheckMode, Pretty: false},
		},
		{
			name:  "persist",
			flags: map[string]any{"output": "build/plugin-index.json", "pretty": false},
			want: pluginIndexCommandRequest{
				Repository: DefaultRepository,
				OutputPath: "build/plugin-index.json",
				Mode:       pluginIndexPersistMode,
				Pretty:     false,
			},
		},
		{
			name:    "conflicting modes",
			flags:   map[string]any{"check": true, "output": "plugin-index.json"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePluginIndexCommandRequest(tt.flags)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parse error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("parse = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestPluginIndexCommandHandlerStopsOnParserFailure(t *testing.T) {
	runner := &recordingPluginIndexCommandRunner{}
	handler := pluginIndexCommandHandler{useCase: runner, clock: fixedPluginIndexClock{}}

	resp, err := handler.Handle(plugin.Request{Flags: map[string]any{"check": true, "output": "plugin-index.json"}})
	if resp != nil || err == nil {
		t.Fatalf("Handle = (%#v, %v), want parser error", resp, err)
	}
	if runner.calls != 0 {
		t.Fatalf("use case called after parser failure: %#v", runner)
	}
}

func TestPluginIndexCommandHandlerInvokesOneUseCaseAndMapsResult(t *testing.T) {
	fixed := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	runner := &recordingPluginIndexCommandRunner{result: pluginIndexCommandResult{
		Repository: "example/project",
		Mode:       pluginIndexCheckMode,
		Plugins:    2,
	}}
	handler := pluginIndexCommandHandler{useCase: runner, clock: fixedPluginIndexClock{now: fixed}}

	resp, err := handler.Handle(plugin.Request{Flags: map[string]any{"check": true, "repository": "example/project"}})
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if runner.calls != 1 || runner.request.Mode != pluginIndexCheckMode || runner.request.Repository != "example/project" {
		t.Fatalf("runner calls/request = %d/%#v", runner.calls, runner.request)
	}
	if resp.Metadata.Command != CommandName || !resp.Metadata.Timestamp.Equal(fixed) {
		t.Fatalf("unexpected metadata: %#v", resp.Metadata)
	}
	want := []map[string]any{
		{"property": "Status", "value": "ok"},
		{"property": "Plugins", "value": 2},
		{"property": "Repository", "value": "example/project"},
	}
	if got := resp.Data["items"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func TestGeneratePluginIndexUseCaseMakesModesAndStopPointsExplicit(t *testing.T) {
	t.Run("query failure stops output work", func(t *testing.T) {
		query := &fakePluginIndexQuerier{err: errors.New("query failed")}
		builder := &fakePluginIndexOutputBuilder{}
		persister := &fakePluginIndexOutputPersister{}
		useCase := newGeneratePluginIndexUseCase(query, builder, persister)

		_, err := useCase.Run(context.Background(), pluginIndexCommandRequest{Mode: pluginIndexRenderMode})
		if err == nil || err.Error() != "query failed" {
			t.Fatalf("Run error = %v", err)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 0, 0)
	})

	t.Run("check is discovery only", func(t *testing.T) {
		query := &fakePluginIndexQuerier{index: sampleCommandIndex()}
		builder := &fakePluginIndexOutputBuilder{output: []byte("must not build")}
		persister := &fakePluginIndexOutputPersister{}
		useCase := newGeneratePluginIndexUseCase(query, builder, persister)

		result, err := useCase.Run(context.Background(), pluginIndexCommandRequest{
			Repository: "example/project",
			Mode:       pluginIndexCheckMode,
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if result.Mode != pluginIndexCheckMode || result.Plugins != 1 || result.Repository != "example/project" {
			t.Fatalf("result = %#v", result)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 0, 0)
	})

	t.Run("builder failure stops persistence", func(t *testing.T) {
		query := &fakePluginIndexQuerier{index: sampleCommandIndex()}
		builder := &fakePluginIndexOutputBuilder{err: errors.New("build failed")}
		persister := &fakePluginIndexOutputPersister{}
		useCase := newGeneratePluginIndexUseCase(query, builder, persister)

		_, err := useCase.Run(context.Background(), pluginIndexCommandRequest{Mode: pluginIndexPersistMode, OutputPath: "plugin-index.json"})
		if err == nil || err.Error() != "build failed" {
			t.Fatalf("Run error = %v", err)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 1, 0)
	})

	t.Run("render returns built bytes without persistence", func(t *testing.T) {
		query := &fakePluginIndexQuerier{index: sampleCommandIndex()}
		builder := &fakePluginIndexOutputBuilder{output: []byte("{\"plugins\":[]}")}
		persister := &fakePluginIndexOutputPersister{}
		useCase := newGeneratePluginIndexUseCase(query, builder, persister)

		result, err := useCase.Run(context.Background(), pluginIndexCommandRequest{Mode: pluginIndexRenderMode, Pretty: true})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if result.RawOutput != "{\"plugins\":[]}" || !builder.options.Pretty {
			t.Fatalf("result/options = %#v/%#v", result, builder.options)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 1, 0)
	})

	t.Run("persist receives complete built bytes", func(t *testing.T) {
		root := t.TempDir()
		query := &fakePluginIndexQuerier{index: sampleCommandIndex()}
		builder := &fakePluginIndexOutputBuilder{output: []byte("complete output")}
		persister := &fakePluginIndexOutputPersister{}
		useCase := newGeneratePluginIndexUseCaseAt(root, query, builder, persister)

		result, err := useCase.Run(context.Background(), pluginIndexCommandRequest{
			Mode:       pluginIndexPersistMode,
			OutputPath: "dist/plugin-index.json",
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		wantPath := filepath.Join(root, "dist", "plugin-index.json")
		if result.OutputPath != "dist/plugin-index.json" || persister.path != wantPath || string(persister.output) != "complete output" {
			t.Fatalf("result/persistence = %#v/%#v", result, persister)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 1, 1)
	})

	t.Run("absolute output path remains a compatibility artifact target", func(t *testing.T) {
		query := &fakePluginIndexQuerier{index: sampleCommandIndex()}
		builder := &fakePluginIndexOutputBuilder{output: []byte("complete output")}
		persister := &fakePluginIndexOutputPersister{}
		outputPath := filepath.Join(t.TempDir(), "plugin-index.json")
		useCase := newGeneratePluginIndexUseCase(query, builder, persister)

		result, err := useCase.Run(context.Background(), pluginIndexCommandRequest{
			Mode:       pluginIndexPersistMode,
			OutputPath: outputPath,
		})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if result.OutputPath != outputPath || persister.path != outputPath || string(persister.output) != "complete output" {
			t.Fatalf("result/persistence = %#v/%#v", result, persister)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 1, 1)
	})

	t.Run("persistence failure is returned", func(t *testing.T) {
		query := &fakePluginIndexQuerier{index: sampleCommandIndex()}
		builder := &fakePluginIndexOutputBuilder{output: []byte("complete output")}
		persister := &fakePluginIndexOutputPersister{err: errors.New("persist failed")}
		useCase := newGeneratePluginIndexUseCase(query, builder, persister)

		_, err := useCase.Run(context.Background(), pluginIndexCommandRequest{
			Mode:       pluginIndexPersistMode,
			OutputPath: "dist/plugin-index.json",
		})
		if err == nil || err.Error() != "persist failed" {
			t.Fatalf("Run error = %v", err)
		}
		assertPluginIndexCalls(t, query, builder, persister, 1, 1, 1)
	})
}

//nolint:govet // Test fake groups returned values before captured request facts.
type recordingPluginIndexCommandRunner struct {
	result  pluginIndexCommandResult
	err     error
	request pluginIndexCommandRequest
	calls   int
}

func (runner *recordingPluginIndexCommandRunner) Run(_ context.Context, request pluginIndexCommandRequest) (pluginIndexCommandResult, error) {
	runner.calls++
	runner.request = request
	return runner.result, runner.err
}

type fixedPluginIndexClock struct{ now time.Time }

func (clock fixedPluginIndexClock) Now() time.Time { return clock.now }

type fakePluginIndexQuerier struct {
	index   *Index
	err     error
	options GenerateOptions
	calls   int
}

func (query *fakePluginIndexQuerier) Query(_ context.Context, options GenerateOptions) (*Index, error) {
	query.calls++
	query.options = options
	if query.index != nil {
		query.index.Repository = options.Repository
	}
	return query.index, query.err
}

type fakePluginIndexOutputBuilder struct {
	err     error
	output  []byte
	options WriteOptions
	calls   int
}

func (builder *fakePluginIndexOutputBuilder) Build(_ *Index, options WriteOptions) ([]byte, error) {
	builder.calls++
	builder.options = options
	return builder.output, builder.err
}

type fakePluginIndexOutputPersister struct {
	err    error
	path   string
	output []byte
	calls  int
}

func (persister *fakePluginIndexOutputPersister) Persist(path string, output []byte) error {
	persister.calls++
	persister.path = path
	persister.output = append([]byte(nil), output...)
	return persister.err
}

func sampleCommandIndex() *Index {
	return &Index{
		SchemaVersion: SchemaVersion,
		Repository:    DefaultRepository,
		Plugins:       []PluginEntry{{Name: "release"}},
	}
}

func assertPluginIndexCalls(
	t *testing.T,
	query *fakePluginIndexQuerier,
	builder *fakePluginIndexOutputBuilder,
	persister *fakePluginIndexOutputPersister,
	wantQuery, wantBuilder, wantPersister int,
) {
	t.Helper()
	if query.calls != wantQuery || builder.calls != wantBuilder || persister.calls != wantPersister {
		t.Fatalf("calls query/builder/persister = %d/%d/%d, want %d/%d/%d", query.calls, builder.calls, persister.calls, wantQuery, wantBuilder, wantPersister)
	}
}
