package releasesource

import (
	"os"
	"strings"
	"testing"
)

func TestTolerantSourceReaderRemainsLocalReadOnlyAndIndependent(t *testing.T) {
	source, err := os.ReadFile("source.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"plugin/release/pkg/release", "net/http", "GITHUB_TOKEN", "TokenResolver",
		"os.WriteFile", "os.Mkdir", "os.Remove", "os.Rename", "os/exec", "exec.Command",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("tolerant source reader contains prohibited capability %q", forbidden)
		}
	}
}
