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

func TestJSONPluginIndexOutputBuilderFreezesSchemaV1BytesEscapingAndNullability(t *testing.T) {
	index := &Index{
		SchemaVersion: SchemaVersion,
		Repository:    "example/<repo>&",
		Plugins: []PluginEntry{{
			Name:        `release"tools`,
			Unit:        "plugin-release",
			Version:     "4.2.0",
			Tag:         "plugin-release/v4.2.0",
			TagPrefix:   "plugin-release/v",
			Manifest:    "plugin/release/manifest.json",
			AssetPrefix: "plugin-release",
			BinaryName:  "plugin-release",
			Description: "line 1\nline <2>",
		}},
	}
	builder := jsonPluginIndexOutputBuilder{}

	compact, err := builder.Build(index, WriteOptions{Pretty: false})
	if err != nil {
		t.Fatalf("Build compact: %v", err)
	}
	wantCompact := `{"schemaVersion":1,"repository":"example/\u003crepo\u003e\u0026","plugins":[{"name":"release\"tools","unit":"plugin-release","version":"4.2.0","tag":"plugin-release/v4.2.0","tagPrefix":"plugin-release/v","manifest":"plugin/release/manifest.json","assetPrefix":"plugin-release","binaryName":"plugin-release","description":"line 1\nline \u003c2\u003e"}]}` + "\n"
	if string(compact) != wantCompact {
		t.Fatalf("compact schema-v1 bytes changed\nwant: %q\n got: %q", wantCompact, compact)
	}

	pretty, err := builder.Build(index, WriteOptions{Pretty: true})
	if err != nil {
		t.Fatalf("Build pretty: %v", err)
	}
	wantPretty := `{
  "schemaVersion": 1,
  "repository": "example/\u003crepo\u003e\u0026",
  "plugins": [
    {
      "name": "release\"tools",
      "unit": "plugin-release",
      "version": "4.2.0",
      "tag": "plugin-release/v4.2.0",
      "tagPrefix": "plugin-release/v",
      "manifest": "plugin/release/manifest.json",
      "assetPrefix": "plugin-release",
      "binaryName": "plugin-release",
      "description": "line 1\nline \u003c2\u003e"
    }
  ]
}
`
	if string(pretty) != wantPretty {
		t.Fatalf("pretty schema-v1 bytes changed\nwant: %q\n got: %q", wantPretty, pretty)
	}

	nilPlugins, err := builder.Build(&Index{SchemaVersion: 1, Repository: "example/project"}, WriteOptions{Pretty: false})
	if err != nil {
		t.Fatalf("Build nil plugins: %v", err)
	}
	if got := string(nilPlugins); got != `{"schemaVersion":1,"repository":"example/project","plugins":null}`+"\n" {
		t.Fatalf("nil collection encoding changed: %q", got)
	}
	emptyPlugins, err := builder.Build(
		&Index{SchemaVersion: 1, Repository: "example/project", Plugins: []PluginEntry{}},
		WriteOptions{Pretty: false},
	)
	if err != nil {
		t.Fatalf("Build empty plugins: %v", err)
	}
	if got := string(emptyPlugins); got != `{"schemaVersion":1,"repository":"example/project","plugins":[]}`+"\n" {
		t.Fatalf("empty collection encoding changed: %q", got)
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
