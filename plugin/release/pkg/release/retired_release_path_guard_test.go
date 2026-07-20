package release

import (
	"strings"
	"testing"
)

func TestRetiredReleasePathsStayRemoved(t *testing.T) {
	cases := []struct {
		path      string
		forbidden []string
	}{
		{
			path: "release_start.go",
			forbidden: []string{
				"func " + "startLegacyRelease(",
			},
		},
		{
			path: "v1_release_application.go",
			forbidden: []string{
				"func " + "newV1ReleaseCommandApplication(",
			},
		},
		{
			path: "command_response.go",
			forbidden: []string{
				"func " + "V2ExecutionUnavailableResponse(",
			},
		},
		{
			path: "release_git_coordinator.go",
			forbidden: []string{
				"func (coordinator *GitReleaseCoordinator) " + "Coordinate(",
			},
		},
		{
			path: "../init/unit_constructor.go",
			forbidden: []string{
				"func " + "buildV2InitConfigFromFlags(",
			},
		},
	}

	for _, tt := range cases {
		source := readCommandBoundarySource(t, tt.path)
		for _, forbidden := range tt.forbidden {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s reintroduced retired release path %q", tt.path, forbidden)
			}
		}
	}
}
