package pluginindex

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestJSONPluginIndexOutputBuilderPreservesStableFormatting(t *testing.T) {
	index := &Index{
		SchemaVersion: SchemaVersion,
		Repository:    "example/project",
		Plugins: []PluginEntry{{
			Name:    "release",
			Unit:    "plugin-release",
			Version: "4.0.2",
			Tag:     "plugin-release/v4.0.2",
		}},
	}
	builder := jsonPluginIndexOutputBuilder{}

	pretty, err := builder.Build(index, WriteOptions{Pretty: true})
	if err != nil {
		t.Fatalf("Build pretty: %v", err)
	}
	if !strings.HasPrefix(string(pretty), "{\n  \"schemaVersion\": 1,") || !strings.HasSuffix(string(pretty), "\n") {
		t.Fatalf("unexpected pretty output: %q", pretty)
	}
	compact, err := builder.Build(index, WriteOptions{Pretty: false})
	if err != nil {
		t.Fatalf("Build compact: %v", err)
	}
	if strings.Count(string(compact), "\n") != 1 || strings.Contains(string(compact), "\n  ") {
		t.Fatalf("unexpected compact output: %q", compact)
	}
	if !strings.Contains(string(compact), `"schemaVersion":1,"repository":"example/project"`) {
		t.Fatalf("compact schema drifted: %q", compact)
	}
}

func TestWriteWithOptionsDelegatesCompleteOutputAndReturnsWriterFailure(t *testing.T) {
	index := &Index{SchemaVersion: SchemaVersion, Repository: DefaultRepository, Plugins: []PluginEntry{}}
	var output bytes.Buffer
	if err := WriteWithOptions(index, &output, WriteOptions{Pretty: false}); err != nil {
		t.Fatalf("WriteWithOptions: %v", err)
	}
	if output.String() != `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[]}`+"\n" {
		t.Fatalf("output = %q", output.String())
	}

	wantErr := errors.New("writer failed")
	if err := Write(index, failingPluginIndexWriter{err: wantErr}); !errors.Is(err, wantErr) {
		t.Fatalf("Write error = %v, want %v", err, wantErr)
	}
}

type failingPluginIndexWriter struct{ err error }

func (writer failingPluginIndexWriter) Write([]byte) (int, error) {
	return 0, writer.err
}
