package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistryLoadsPluginIndexFromPluginRegistryRelease(t *testing.T) {
	registry, _, paths := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   true,
	})

	plugins, err := registry.FetchAvailablePlugins()
	if err != nil {
		t.Fatalf("FetchAvailablePlugins returned error: %v", err)
	}

	want := []AvailablePlugin{
		{Name: "release", Version: "4.0.3", Description: "Release management plugin"},
		{Name: "ui", Version: "1.0.1", Description: "UI component helper plugin"},
	}
	if len(plugins) != len(want) {
		t.Fatalf("plugins = %#v, want %#v", plugins, want)
	}
	for i := range want {
		if plugins[i] != want[i] {
			t.Fatalf("plugin[%d] = %#v, want %#v", i, plugins[i], want[i])
		}
	}

	assertPathCalled(t, *paths, "/releases/tags/plugin-registry")
	assertPathCalled(t, *paths, "/assets/plugin-index.json")
	assertNoReleaseListCall(t, *paths)
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistryIndexAllowsNewPluginWithoutCodeMapping(t *testing.T) {
	index := `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"deploy","unit":"plugin-deploy","version":"1.2.3","tag":"plugin-deploy/v1.2.3","tagPrefix":"plugin-deploy/v","manifest":"plugin/deploy/manifest.json","assetPrefix":"plugin-deploy","binaryName":"plugin-deploy","description":"Deploy plugin"}]}`
	registry, _, _ := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           index,
		includeIndexRelease: true,
		includeIndexAsset:   true,
	})

	plugins, err := registry.FetchAvailablePlugins()
	if err != nil {
		t.Fatalf("FetchAvailablePlugins returned error: %v", err)
	}
	if len(plugins) != 1 || plugins[0].Name != "deploy" || plugins[0].Version != "1.2.3" {
		t.Fatalf("plugins = %#v, want deploy 1.2.3", plugins)
	}

	version, err := registry.GetPluginVersion("deploy")
	if err != nil {
		t.Fatalf("GetPluginVersion returned error: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", version)
	}

	tag, err := registry.GetLatestVersion("deploy")
	if err != nil {
		t.Fatalf("GetLatestVersion returned error: %v", err)
	}
	if tag != "plugin-deploy/v1.2.3" {
		t.Fatalf("tag = %q, want plugin-deploy/v1.2.3", tag)
	}
}

func TestRegistryPluginIndexMissingReleaseFailsClearlyWithoutFallback(t *testing.T) {
	registry, _, paths := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: false,
		includeIndexAsset:   true,
	})

	_, err := registry.FetchAvailablePlugins()
	if err == nil {
		t.Fatal("expected missing index release error")
	}
	if !strings.Contains(err.Error(), "failed to load plugin index from plugin-registry release: release or asset not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoReleaseListCall(t, *paths)
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistryPluginIndexMissingAssetFailsClearlyWithoutFallback(t *testing.T) {
	registry, _, paths := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   false,
	})

	_, err := registry.FetchAvailablePlugins()
	if err == nil {
		t.Fatal("expected missing index asset error")
	}
	if !strings.Contains(err.Error(), "failed to load plugin index from plugin-registry release: release or asset not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	assertNoReleaseListCall(t, *paths)
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistryRejectsInvalidPluginIndexes(t *testing.T) {
	tests := []struct {
		name    string
		index   string
		wantErr string
	}{
		{name: "malformed", index: `{"schemaVersion":`, wantErr: "malformed JSON"},
		{name: "schema", index: `{"schemaVersion":2,"repository":"nekoman-hq/neko-cli","plugins":[]}`, wantErr: "schemaVersion must be 1"},
		{name: "duplicate plugin", index: `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"release","unit":"plugin-release","version":"4.0.3","tag":"plugin-release/v4.0.3","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"},{"name":"release","unit":"plugin-release-2","version":"4.0.4","tag":"plugin-release/v4.0.4","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"}]}`, wantErr: `duplicate plugin name "release"`},
		{name: "invalid semver", index: `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"release","unit":"plugin-release","version":"v4.0.3","tag":"plugin-release/v4.0.3","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"}]}`, wantErr: `version "v4.0.3" is not valid semver`},
		{name: "tag mismatch", index: `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"release","unit":"plugin-release","version":"4.0.3","tag":"plugin-release/v4.0.2","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"}]}`, wantErr: `tag "plugin-release/v4.0.2" must equal tagPrefix + version`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, _, paths := newPluginIndexRegistryTestServer(t, registryServerOptions{
				indexJSON:           tt.index,
				includeIndexRelease: true,
				includeIndexAsset:   true,
			})

			_, err := registry.FetchAvailablePlugins()
			if err == nil {
				t.Fatal("expected invalid index error")
			}
			if !strings.Contains(err.Error(), "invalid plugin index") || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tt.wantErr)
			}
			assertNoReleaseListCall(t, *paths)
			assertNoLatestReleaseCall(t, *paths)
		})
	}
}

func TestRegistryUnknownPluginListsIndexPlugins(t *testing.T) {
	registry, _, _ := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   true,
	})

	_, err := registry.GetPluginVersion("deploy")
	if err == nil {
		t.Fatal("expected unknown plugin error")
	}
	if !strings.Contains(err.Error(), `unknown plugin "deploy"; available plugins: release, ui`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryVersionResolutionUsesPluginIndex(t *testing.T) {
	registry, _, _ := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   true,
	})

	tests := []struct {
		version string
		want    string
	}{
		{version: "latest", want: "plugin-release/v4.0.3"},
		{version: "", want: "plugin-release/v4.0.3"},
		{version: "4.0.2", want: "plugin-release/v4.0.2"},
		{version: "v4.0.2", want: "plugin-release/v4.0.2"},
		{version: "plugin-release/v4.0.1", want: "plugin-release/v4.0.1"},
	}

	for _, tt := range tests {
		got, err := registry.ResolveReleaseTag("release", tt.version)
		if err != nil {
			t.Fatalf("ResolveReleaseTag(%q) returned error: %v", tt.version, err)
		}
		if got != tt.want {
			t.Fatalf("ResolveReleaseTag(%q) = %q, want %q", tt.version, got, tt.want)
		}
	}

	_, err := registry.ResolveReleaseTag("release", "plugin-ui/v1.0.1")
	if err == nil {
		t.Fatal("expected wrong plugin tag error")
	}
	if !strings.Contains(err.Error(), `release tag "plugin-ui/v1.0.1" does not belong to plugin "release"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryDownloadURLUsesIndexAssetPrefixAndExactTagRelease(t *testing.T) {
	releases := map[string]registryRelease{
		"plugin-release/v4.0.3": {
			TagName: "plugin-release/v4.0.3",
			Assets: []registryAsset{
				{Name: "neko-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/cli.tar.gz"},
				{Name: "plugin-ui_1.0.1_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/ui.tar.gz"},
				{Name: "plugin-release_4.0.3_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/release.tar.gz"},
			},
		},
	}
	registry, _, paths := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   true,
		tagReleases:         releases,
	})

	downloadURL, err := registry.GetDownloadURL("release", "plugin-release/v4.0.3", "Darwin", "arm64")
	if err != nil {
		t.Fatalf("GetDownloadURL returned error: %v", err)
	}
	if downloadURL != "https://example.com/release.tar.gz" {
		t.Fatalf("downloadURL = %q, want release asset", downloadURL)
	}

	assertPathCalled(t, *paths, "/releases/tags/plugin-registry")
	assertPathCalled(t, *paths, "/assets/plugin-index.json")
	assertPathCalled(t, *paths, "/releases/tags/plugin-release%2Fv4.0.3")
	assertNoReleaseListCall(t, *paths)
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistryDownloadURLRejectsExactTagForWrongPlugin(t *testing.T) {
	registry, _, paths := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   true,
		tagReleases: map[string]registryRelease{
			"plugin-ui/v1.0.1": {TagName: "plugin-ui/v1.0.1"},
		},
	})

	_, err := registry.GetDownloadURL("release", "plugin-ui/v1.0.1", "Darwin", "arm64")
	if err == nil {
		t.Fatal("expected wrong plugin tag error")
	}
	if !strings.Contains(err.Error(), `release tag "plugin-ui/v1.0.1" does not belong to plugin "release"`) {
		t.Fatalf("unexpected error: %v", err)
	}
	assertPathNotCalled(t, *paths, "/releases/tags/plugin-ui%2Fv1.0.1")
	assertNoReleaseListCall(t, *paths)
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistryDownloadURLMissingIndexedAssetFailsClearly(t *testing.T) {
	releases := map[string]registryRelease{
		"plugin-release/v4.0.3": {
			TagName: "plugin-release/v4.0.3",
			Assets: []registryAsset{
				{Name: "neko-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/cli.tar.gz"},
				{Name: "plugin-release_4.0.3_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/release.tar.gz"},
				{Name: "plugin-ui_1.0.1_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/ui.tar.gz"},
			},
		},
	}
	registry, _, _ := newPluginIndexRegistryTestServer(t, registryServerOptions{
		indexJSON:           validPluginIndexJSON(),
		includeIndexRelease: true,
		includeIndexAsset:   true,
		tagReleases:         releases,
	})

	_, err := registry.GetDownloadURL("release", "plugin-release/v4.0.3", "Darwin", "arm64")
	if err == nil {
		t.Fatal("expected missing asset error")
	}
	message := err.Error()
	for _, want := range []string{
		`plugin "release" release "plugin-release/v4.0.3" has no asset for Darwin/arm64`,
		"neko-cli_Darwin_arm64.tar.gz",
		"plugin-release_4.0.3_Linux_x86_64.tar.gz",
		"plugin-ui_1.0.1_Darwin_arm64.tar.gz",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestRegistryCodeContainsNoLegacyFallbackSymbols(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "pkg/plugin/registry.go"))
	if err != nil {
		t.Fatalf("read registry.go: %v", err)
	}
	source := string(data)
	for _, forbidden := range []string{
		"temporaryBuiltinPluginFallbacks",
		"TODO(M2E)",
		"pluginReleaseIdentities",
		"pluginReleaseIdentity",
		"selectLatestPluginRelease",
		"latestPluginRelease",
		"selectLatestTemporaryFallbackRelease",
		"maxReleasePages",
		"listReleases",
		"releases?per_page",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("registry.go must not contain legacy fallback symbol %q", forbidden)
		}
	}
}

//nolint:govet // Test helper groups HTTP fixture options by meaning.
type registryServerOptions struct {
	indexJSON           string
	tagReleases         map[string]registryRelease
	includeIndexRelease bool
	includeIndexAsset   bool
}

func newPluginIndexRegistryTestServer(t *testing.T, opts registryServerOptions) (*Registry, *httptest.Server, *[]string) {
	t.Helper()

	var server *httptest.Server
	paths := []string{}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.String())

		if r.URL.Path == "/assets/plugin-index.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(opts.indexJSON))
			return
		}

		if r.URL.Path == "/releases" {
			http.NotFound(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/releases/tags/") {
			tag, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/releases/tags/"))
			if err != nil {
				t.Fatalf("unescape tag: %v", err)
			}
			if tag == DefaultPluginIndexReleaseTag {
				if !opts.includeIndexRelease {
					http.NotFound(w, r)
					return
				}
				assets := []registryAsset{}
				if opts.includeIndexAsset {
					assets = append(assets, registryAsset{Name: DefaultPluginIndexAssetName, BrowserDownloadURL: server.URL + "/assets/plugin-index.json"})
				}
				writeJSON(t, w, registryRelease{TagName: DefaultPluginIndexReleaseTag, Assets: assets})
				return
			}
			release, ok := opts.tagReleases[tag]
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, release)
			return
		}

		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	return NewRegistryWithURL(server.URL + "/releases"), server, &paths
}

func validPluginIndexJSON() string {
	return `{
  "schemaVersion": 1,
  "repository": "nekoman-hq/neko-cli",
  "plugins": [
    {
      "name": "ui",
      "unit": "plugin-ui",
      "version": "1.0.1",
      "tag": "plugin-ui/v1.0.1",
      "tagPrefix": "plugin-ui/v",
      "manifest": "plugin/ui/manifest.json",
      "assetPrefix": "plugin-ui",
      "binaryName": "plugin-ui",
      "description": "UI component helper plugin"
    },
    {
      "name": "release",
      "unit": "plugin-release",
      "version": "4.0.3",
      "tag": "plugin-release/v4.0.3",
      "tagPrefix": "plugin-release/v",
      "manifest": "plugin/release/manifest.json",
      "assetPrefix": "plugin-release",
      "binaryName": "plugin-release",
      "description": "Release management plugin"
    }
  ]
}`
}

func assertPathCalled(t *testing.T, paths []string, wantPath string) {
	t.Helper()
	for _, called := range paths {
		if strings.HasPrefix(called, wantPath) {
			return
		}
	}
	t.Fatalf("expected path %s to be called; paths=%v", wantPath, paths)
}

func assertPathNotCalled(t *testing.T, paths []string, wantPath string) {
	t.Helper()
	for _, called := range paths {
		if strings.HasPrefix(called, wantPath) {
			t.Fatalf("expected path %s not to be called; paths=%v", wantPath, paths)
		}
	}
}

func assertNoLatestReleaseCall(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if strings.Contains(path, "/latest") {
			t.Fatalf("registry must not call repository latest release endpoint, called %s", path)
		}
	}
}

func assertNoReleaseListCall(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if path == "/releases" || strings.Contains(path, "releases?per_page") {
			t.Fatalf("registry must not call release-list endpoint for plugin discovery, called %s", path)
		}
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
