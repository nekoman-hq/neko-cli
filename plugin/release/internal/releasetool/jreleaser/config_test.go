package jreleaser

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveAndLoadConfigAtExplicitRoot(t *testing.T) {
	root := t.TempDir()
	labels := []string{"feat"}
	config := &Config{
		Project: Project{
			Name: "example", Version: "1.2.3", Authors: &[]string{"Neko"},
			Languages: ProjectLanguages{Java: JavaLanguage{GroupID: "at.example", Version: "25"}},
			Links:     ProjectLinks{Homepage: "https://example.test"},
		},
		Release: Release{Github: GithubRelease{
			Owner: "example", Name: "project",
			Changelog: Changelog{IncludeLabels: &labels, SkipMergeCommits: true},
		}},
	}
	if err := SaveConfigAt(root, config); err != nil {
		t.Fatalf("SaveConfigAt: %v", err)
	}
	loaded, err := LoadConfigAt(root)
	if err != nil {
		t.Fatalf("LoadConfigAt: %v", err)
	}
	if !reflect.DeepEqual(loaded, config) {
		t.Fatalf("loaded config = %#v, want %#v", loaded, config)
	}
	content, err := os.ReadFile(filepath.Join(root, "jreleaser.yml"))
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	for _, key := range []string{"includeLabels:", "skipMergeCommits:"} {
		if !strings.Contains(string(content), key) {
			t.Errorf("saved config omits %q:\n%s", key, content)
		}
	}
}
