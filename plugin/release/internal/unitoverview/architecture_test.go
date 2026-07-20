package unitoverview

import (
	"os"
	"strings"
	"testing"
)

func TestUnitOverviewCapabilityBoundary(t *testing.T) {
	for _, path := range []string{
		"boundary.go", "unit_overview_command.go", "unit_overview_inspection.go",
		"unit_overview_response.go", "unit_overview_source.go", "unit_overview_types.go",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(source)
		for _, forbidden := range []string{
			"plugin/release/pkg/release", "net/http", "os/exec", "exec.Command",
			"os.WriteFile", "os.Mkdir", "os.Chdir", "os.Setenv", "os.Getenv", "os.LookupEnv",
			"HandleDoctor", "HandleGitHubWorkflowInit", "HandleReleaseContextValidation",
		} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains prohibited dependency %q", path, forbidden)
			}
		}
	}
}
