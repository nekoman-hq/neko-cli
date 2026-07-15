package main

import (
	"os"
	"strings"
	"testing"
)

func TestReadOnlyQueryCommandBoundariesRemainSeparated(t *testing.T) {
	commands := []struct {
		name       string
		handler    string
		query      string
		parserCall string
		queryCall  string
		mapperCall string
	}{
		{
			name:       "validate",
			handler:    "pkg/validate/validate.go",
			query:      "pkg/validate/query.go",
			parserCall: "parseValidationQueryRequest(req.Flags)",
			queryCall:  "handler.query.Query(",
			mapperCall: "mapValidationQueryResponse(",
		},
		{
			name:       "history",
			handler:    "pkg/history/history.go",
			query:      "pkg/history/query.go",
			parserCall: "parseHistoryQueryRequest(req.Flags)",
			queryCall:  "handler.query.Query(",
			mapperCall: "mapHistoryQueryResponse(",
		},
		{
			name:       "contributors",
			handler:    "pkg/contributors/contributors.go",
			query:      "pkg/contributors/query.go",
			parserCall: "parseContributorsQueryRequest(req.Flags)",
			queryCall:  "handler.query.Query(",
			mapperCall: "mapContributorsQueryResponse(",
		},
	}

	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			handler := readQueryArchitectureFile(t, command.handler)
			for _, required := range []string{command.parserCall, command.queryCall, command.mapperCall} {
				if !strings.Contains(handler, required) {
					t.Fatalf("%s must contain %q", command.handler, required)
				}
			}
			for _, forbidden := range []string{
				"LoadReleaseRepository(",
				"ResolveReleaseUnit(",
				"exec.Command(",
				"os.WriteFile(",
				"os.MkdirAll(",
				"&plugin.Response{",
			} {
				if strings.Contains(handler, forbidden) {
					t.Fatalf("%s must not contain %q", command.handler, forbidden)
				}
			}

			query := readQueryArchitectureFile(t, command.query)
			for _, forbidden := range []string{
				"github.com/nekoman-hq/neko-cli/pkg/plugin",
				"plugin.Response",
				"exec.Command(",
				"os.WriteFile(",
				"os.MkdirAll(",
				"AtomicWrite",
				"map[string]any",
			} {
				if strings.Contains(query, forbidden) {
					t.Fatalf("%s must not contain %q", command.query, forbidden)
				}
			}
		})
	}
}

func TestReadOnlyQueryExtractionIntroducesNoGenericManager(t *testing.T) {
	for _, path := range []string{
		"pkg/validate/query.go",
		"pkg/history/query.go",
		"pkg/contributors/query.go",
		"pkg/pluginindex/index.go",
		"pkg/pluginindex/command_use_case.go",
		"pkg/pluginindex/output_builder.go",
		"pkg/pluginindex/output_persister.go",
	} {
		source := readQueryArchitectureFile(t, path)
		for _, forbidden := range []string{
			"QueryService",
			"ReadService",
			"InspectionManager",
			"ReleaseQueryCoordinator",
			"QueryRegistry",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s introduces forbidden generic abstraction %q", path, forbidden)
			}
		}
	}
}

func TestPluginIndexDiscoveryBuildingAndPersistenceRemainSeparated(t *testing.T) {
	handler := readQueryArchitectureFile(t, "pkg/pluginindex/handler.go")
	for _, required := range []string{
		"parsePluginIndexCommandRequest(req.Flags)",
		"handler.useCase.Run(",
		"mapPluginIndexCommandResponse(",
	} {
		if !strings.Contains(handler, required) {
			t.Fatalf("plugin-index handler must contain %q", required)
		}
	}
	for _, forbidden := range []string{
		"Generate(",
		".Build(",
		".Persist(",
		"os.WriteFile(",
		"os.MkdirAll(",
		"&plugin.Response{",
	} {
		if strings.Contains(handler, forbidden) {
			t.Fatalf("plugin-index handler must not contain %q", forbidden)
		}
	}

	useCase := readQueryArchitectureFile(t, "pkg/pluginindex/command_use_case.go")
	for _, required := range []string{"useCase.query.Query(", "useCase.builder.Build(", "useCase.persister.Persist("} {
		if !strings.Contains(useCase, required) {
			t.Fatalf("plugin-index command use case must contain %q", required)
		}
	}
	for _, forbidden := range []string{"pkg/plugin", "plugin.Response", "map[string]any", "os.WriteFile(", "os.MkdirAll(", "json.NewEncoder("} {
		if strings.Contains(useCase, forbidden) {
			t.Fatalf("plugin-index command use case must not contain %q", forbidden)
		}
	}

	query := readQueryArchitectureFile(t, "pkg/pluginindex/index.go")
	for _, forbidden := range []string{"pkg/plugin", "plugin.Response", "os.WriteFile(", "os.MkdirAll(", "AtomicWrite"} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("plugin-index query must not contain %q", forbidden)
		}
	}

	builder := readQueryArchitectureFile(t, "pkg/pluginindex/output_builder.go")
	for _, forbidden := range []string{"\"os\"", "\"path/filepath\"", "pkg/config", "pkg/plugin", "plugin.Response", "Persist("} {
		if strings.Contains(builder, forbidden) {
			t.Fatalf("plugin-index output builder must not contain %q", forbidden)
		}
	}

	persister := readQueryArchitectureFile(t, "pkg/pluginindex/output_persister.go")
	for _, forbidden := range []string{"V2ReleaseConfig", "V2ReleaseState", "pluginManifest", "PluginEntry", "Generate(", "plugin.Response"} {
		if strings.Contains(persister, forbidden) {
			t.Fatalf("plugin-index persister must not contain %q", forbidden)
		}
	}
}

func readQueryArchitectureFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
