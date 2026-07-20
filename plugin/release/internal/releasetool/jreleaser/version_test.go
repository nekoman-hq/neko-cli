package jreleaser

import (
	"bytes"
	"strings"
	"testing"
)

func TestRewriteProjectVersionPreservesSurroundingBytes(t *testing.T) {
	before := []byte("project:\r\n  name: example\r\n  version: 1.2.3\r\n  languages:\r\n    java:\r\n      version: 25\r\nrelease:\r\n  github:\r\n    name: example\r\n")
	want := bytes.Replace(before, []byte("  version: 1.2.3\r\n"), []byte("  version: 2.0.0\r\n"), 1)
	got, err := RewriteProjectVersion(before, "2.0.0")
	if err != nil {
		t.Fatalf("RewriteProjectVersion: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rewritten config:\n%s\nwant:\n%s", got, want)
	}
}

func TestRewriteProjectVersionRejectsMissingProjectVersion(t *testing.T) {
	_, err := RewriteProjectVersion([]byte("project:\n  name: example\n"), "2.0.0")
	if err == nil || !strings.Contains(err.Error(), "locate project.version in jreleaser.yml: version not found") {
		t.Fatalf("missing version error = %v", err)
	}
}
