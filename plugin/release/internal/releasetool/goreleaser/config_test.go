package goreleaser

import (
	"reflect"
	"testing"
)

func TestParseConfigAndExactLookup(t *testing.T) {
	config, err := ParseConfig([]byte(`version: 2
project_name: neko
builds:
  - id: cli
    binary: neko
    main: .
    goos: [darwin, linux, windows]
archives:
  - id: cli
    ids: [cli]
    formats: [tar.gz]
    name_template: 'cli_{{ .Os }}_{{ .Arch }}'
checksum:
  name_template: cli_{{ .Version }}_checksums.txt
release:
  ids: [cli]
before:
  hooks: [go mod tidy]
`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if config.Version != 2 || config.ProjectName != "neko" {
		t.Fatalf("unexpected config: %#v", config)
	}
	if build, ok := BuildByID(config.Builds, "cli"); !ok || build.Binary != "neko" {
		t.Fatalf("build lookup = %#v, %t", build, ok)
	}
	if archive, ok := ArchiveByID(config.Archives, "cli"); !ok || !Contains(archive.IDs, "cli") {
		t.Fatalf("archive lookup = %#v, %t", archive, ok)
	}
}

func TestParseConfigRejectsInvalidAndMultipleDocuments(t *testing.T) {
	if _, err := ParseConfig([]byte("version: [two\n")); err == nil {
		t.Fatal("invalid YAML was accepted")
	}
	if _, err := ParseConfig([]byte("version: 2\n---\nversion: 2\n")); err == nil {
		t.Fatal("multiple YAML documents were accepted")
	}
	if config, err := ParseConfig(nil); err != nil || !reflect.DeepEqual(config, Config{}) {
		t.Fatalf("empty config = %#v, %v", config, err)
	}
}
