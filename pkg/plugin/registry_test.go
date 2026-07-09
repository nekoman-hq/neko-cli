package plugin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestRegistryLoadsPluginIndexFromPluginRegistryRelease(t *testing.T) {
	registry, _, paths := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), map[string]registryRelease{}, nil)

	plugins, err := registry.FetchAvailablePlugins()
	if err != nil {
		t.Fatalf("FetchAvailablePlugins returned error: %v", err)
	}

	want := []AvailablePlugin{
		{Name: "release", Version: "4.0.2", Description: "Release management plugin"},
		{Name: "ui", Version: "1.0.0", Description: "UI component helper plugin"},
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
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistryIndexAllowsNewPluginWithoutCodeMapping(t *testing.T) {
	index := `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"deploy","unit":"plugin-deploy","version":"1.2.3","tag":"plugin-deploy/v1.2.3","tagPrefix":"plugin-deploy/v","manifest":"plugin/deploy/manifest.json","assetPrefix":"plugin-deploy","binaryName":"plugin-deploy","description":"Deploy plugin"}]}`
	registry, _, _ := newPluginIndexRegistryTestServer(t, index, map[string]registryRelease{}, nil)

	version, err := registry.GetPluginVersion("deploy")
	if err != nil {
		t.Fatalf("GetPluginVersion returned error: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("version = %q, want 1.2.3", version)
	}
	tag, err := registry.ResolveReleaseTag("deploy", "latest")
	if err != nil {
		t.Fatalf("ResolveReleaseTag returned error: %v", err)
	}
	if tag != "plugin-deploy/v1.2.3" {
		t.Fatalf("tag = %q, want plugin-deploy/v1.2.3", tag)
	}
}

func TestRegistryPluginIndexMissingReleaseFailsClearlyWhenLoadedDirectly(t *testing.T) {
	registry, _, _ := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), map[string]registryRelease{}, nil)
	registry.indexReleaseTag = "missing-registry"

	_, err := registry.loadPluginIndex()
	if err == nil {
		t.Fatal("expected missing index release error")
	}
	if !strings.Contains(err.Error(), "failed to load plugin index from plugin-registry release") ||
		!strings.Contains(err.Error(), "404") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryPluginIndexMissingAssetFailsClearlyWhenLoadedDirectly(t *testing.T) {
	registry, _, _ := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), map[string]registryRelease{}, nil)
	registry.indexAssetName = "missing.json"

	_, err := registry.loadPluginIndex()
	if err == nil {
		t.Fatal("expected missing index asset error")
	}
	if !strings.Contains(err.Error(), `asset "missing.json" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryRejectsInvalidPluginIndexes(t *testing.T) {
	tests := []struct {
		name    string
		index   string
		wantErr string
	}{
		{name: "malformed", index: `{"schemaVersion":`, wantErr: "malformed JSON"},
		{name: "schema", index: `{"schemaVersion":2,"repository":"nekoman-hq/neko-cli","plugins":[]}`, wantErr: "schemaVersion must be 1"},
		{name: "duplicate plugin", index: `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"release","unit":"plugin-release","version":"4.0.2","tag":"plugin-release/v4.0.2","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"},{"name":"release","unit":"plugin-release-2","version":"4.0.3","tag":"plugin-release/v4.0.3","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"}]}`, wantErr: `duplicate plugin name "release"`},
		{name: "invalid semver", index: `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"release","unit":"plugin-release","version":"v4.0.2","tag":"plugin-release/v4.0.2","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"}]}`, wantErr: `version "v4.0.2" is not valid semver`},
		{name: "tag mismatch", index: `{"schemaVersion":1,"repository":"nekoman-hq/neko-cli","plugins":[{"name":"release","unit":"plugin-release","version":"4.0.2","tag":"plugin-release/v4.0.1","tagPrefix":"plugin-release/v","assetPrefix":"plugin-release","binaryName":"plugin-release"}]}`, wantErr: `tag "plugin-release/v4.0.1" must equal tagPrefix + version`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry, _, _ := newPluginIndexRegistryTestServer(t, tt.index, map[string]registryRelease{}, nil)
			_, err := registry.FetchAvailablePlugins()
			if err == nil {
				t.Fatal("expected invalid index error")
			}
			if !strings.Contains(err.Error(), "invalid plugin index") || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRegistryUnknownPluginListsIndexPlugins(t *testing.T) {
	registry, _, _ := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), map[string]registryRelease{}, nil)

	_, err := registry.GetPluginVersion("deploy")
	if err == nil {
		t.Fatal("expected unknown plugin error")
	}
	if !strings.Contains(err.Error(), `unknown plugin "deploy"; available plugins: release, ui`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryVersionResolutionUsesPluginIndex(t *testing.T) {
	registry, _, _ := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), map[string]registryRelease{}, nil)

	tests := []struct {
		version string
		want    string
	}{
		{version: "latest", want: "plugin-release/v4.0.2"},
		{version: "", want: "plugin-release/v4.0.2"},
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

	_, err := registry.ResolveReleaseTag("release", "plugin-ui/v1.0.0")
	if err == nil {
		t.Fatal("expected wrong plugin tag error")
	}
	if !strings.Contains(err.Error(), `does not belong to plugin "release"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegistryDownloadURLUsesIndexAssetPrefix(t *testing.T) {
	releases := map[string]registryRelease{
		"plugin-release/v4.0.2": {
			TagName: "plugin-release/v4.0.2",
			Assets: []registryAsset{
				{Name: "neko-cli_4.0.2_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/cli.tar.gz"},
				{Name: "plugin-ui_4.0.2_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/ui.tar.gz"},
				{Name: "plugin-release_4.0.2_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/release.tar.gz"},
			},
		},
	}
	registry, _, _ := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), releases, nil)

	downloadURL, err := registry.GetDownloadURL("release", "plugin-release/v4.0.2", "Darwin", "arm64")
	if err != nil {
		t.Fatalf("GetDownloadURL returned error: %v", err)
	}
	if downloadURL != "https://example.com/release.tar.gz" {
		t.Fatalf("downloadURL = %q, want release asset", downloadURL)
	}
}

func TestRegistryDownloadURLMissingIndexedAssetFailsClearly(t *testing.T) {
	releases := map[string]registryRelease{
		"plugin-release/v4.0.2": {
			TagName: "plugin-release/v4.0.2",
			Assets: []registryAsset{
				{Name: "plugin-release_4.0.2_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/release.tar.gz"},
				{Name: "plugin-ui_4.0.2_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/ui.tar.gz"},
			},
		},
	}
	registry, _, _ := newPluginIndexRegistryTestServer(t, validPluginIndexJSON(), releases, nil)

	_, err := registry.GetDownloadURL("release", "plugin-release/v4.0.2", "Darwin", "arm64")
	if err == nil {
		t.Fatal("expected missing asset error")
	}
	message := err.Error()
	for _, want := range []string{`plugin "release" release "plugin-release/v4.0.2" has no asset for Darwin/arm64`, "plugin-release_4.0.2_Linux_x86_64.tar.gz", "plugin-ui_4.0.2_Darwin_arm64.tar.gz"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestRegistryTemporaryFallbackWhenPluginIndexMissing(t *testing.T) {
	registry, _, paths := newPluginIndexRegistryTestServer(t, "", map[string]registryRelease{}, [][]registryRelease{
		{
			{TagName: "v3.0.2"},
			{TagName: "plugin-ui/v1.0.1"},
			{TagName: "plugin-release/v4.0.2"},
		},
	})
	registry.indexReleaseTag = "missing-registry"

	plugins, err := registry.FetchAvailablePlugins()
	if err != nil {
		t.Fatalf("FetchAvailablePlugins returned error: %v", err)
	}

	want := []AvailablePlugin{
		{Name: "release", Version: "4.0.2"},
		{Name: "ui", Version: "1.0.1"},
	}
	if len(plugins) != len(want) {
		t.Fatalf("plugins = %#v, want %#v", plugins, want)
	}
	for i := range want {
		if plugins[i] != want[i] {
			t.Fatalf("plugin[%d] = %#v, want %#v", i, plugins[i], want[i])
		}
	}
	assertNoLatestReleaseCall(t, *paths)
}

func TestRegistrySelectsLatestPluginReleaseByTagPrefix(t *testing.T) {
	registry, _, paths := newRegistryTestServer(t, [][]registryRelease{
		{
			{TagName: "v3.0.2"},
			{TagName: "plugin-ui/v9.0.0"},
			{TagName: "plugin-release/v4.0.2"},
			{TagName: "plugin-release/v4.0.10"},
			{TagName: "plugin-release/foo"},
			{TagName: "plugin-release/v4.1.0", Draft: true},
			{TagName: "plugin-release/v4.2.0", PreRelease: true},
		},
	}, nil)

	version, err := registry.GetPluginVersion("release")
	if err != nil {
		t.Fatalf("GetPluginVersion returned error: %v", err)
	}
	if version != "4.0.10" {
		t.Fatalf("GetPluginVersion = %q, want 4.0.10", version)
	}

	tag, err := registry.GetLatestVersion("release")
	if err != nil {
		t.Fatalf("GetLatestVersion returned error: %v", err)
	}
	if tag != "plugin-release/v4.0.10" {
		t.Fatalf("GetLatestVersion = %q, want plugin-release/v4.0.10", tag)
	}

	for _, path := range *paths {
		if strings.Contains(path, "/latest") {
			t.Fatalf("registry must not call repository latest release endpoint, called %s", path)
		}
	}
}

func TestRegistrySelectsRequestedPluginUnit(t *testing.T) {
	registry, _, _ := newRegistryTestServer(t, [][]registryRelease{
		{
			{TagName: "plugin-ui/v1.0.1"},
			{TagName: "plugin-release/v4.0.2"},
		},
	}, nil)

	releaseVersion, err := registry.GetPluginVersion("release")
	if err != nil {
		t.Fatalf("GetPluginVersion(release) returned error: %v", err)
	}
	if releaseVersion != "4.0.2" {
		t.Fatalf("GetPluginVersion(release) = %q, want 4.0.2", releaseVersion)
	}

	uiVersion, err := registry.GetPluginVersion("ui")
	if err != nil {
		t.Fatalf("GetPluginVersion(ui) returned error: %v", err)
	}
	if uiVersion != "1.0.1" {
		t.Fatalf("GetPluginVersion(ui) = %q, want 1.0.1", uiVersion)
	}
}

func TestRegistryFetchAvailablePluginsUsesPluginSpecificReleases(t *testing.T) {
	registry, _, _ := newRegistryTestServer(t, [][]registryRelease{
		{
			{TagName: "v3.0.2"},
			{TagName: "plugin-ui/v1.0.1"},
			{TagName: "plugin-release/v4.0.2"},
		},
	}, nil)

	plugins, err := registry.FetchAvailablePlugins()
	if err != nil {
		t.Fatalf("FetchAvailablePlugins returned error: %v", err)
	}

	want := []AvailablePlugin{
		{Name: "release", Version: "4.0.2"},
		{Name: "ui", Version: "1.0.1"},
	}
	if len(plugins) != len(want) {
		t.Fatalf("FetchAvailablePlugins returned %d plugins, want %d: %#v", len(plugins), len(want), plugins)
	}
	for i := range want {
		if plugins[i] != want[i] {
			t.Fatalf("plugin[%d] = %#v, want %#v", i, plugins[i], want[i])
		}
	}
}

func TestRegistryFindsPluginReleaseOnSecondPage(t *testing.T) {
	registry, _, _ := newRegistryTestServer(t, [][]registryRelease{
		{{TagName: "v3.0.2"}},
		{{TagName: "plugin-release/v4.0.2"}},
	}, nil)

	version, err := registry.GetPluginVersion("release")
	if err != nil {
		t.Fatalf("GetPluginVersion returned error: %v", err)
	}
	if version != "4.0.2" {
		t.Fatalf("GetPluginVersion = %q, want 4.0.2", version)
	}
}

func TestRegistryDownloadURLSelectsPluginAssetFromSelectedRelease(t *testing.T) {
	release := registryRelease{
		TagName: "plugin-release/v4.0.2",
		Assets: []registryAsset{
			{Name: "neko-cli_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/cli.tar.gz"},
			{Name: "plugin-ui_1.0.1_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/ui.tar.gz"},
			{Name: "plugin-release_4.0.2_Darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/release.tar.gz"},
		},
	}
	registry, _, _ := newRegistryTestServer(t, nil, map[string]registryRelease{
		"plugin-release/v4.0.2": release,
	})

	downloadURL, err := registry.GetDownloadURL("release", "plugin-release/v4.0.2", "Darwin", "arm64")
	if err != nil {
		t.Fatalf("GetDownloadURL returned error: %v", err)
	}
	if downloadURL != "https://example.com/release.tar.gz" {
		t.Fatalf("GetDownloadURL = %q, want release asset URL", downloadURL)
	}
}

func TestRegistryDownloadURLMissingPlatformFailsClearly(t *testing.T) {
	release := registryRelease{
		TagName: "plugin-release/v4.0.2",
		Assets: []registryAsset{
			{Name: "plugin-release_4.0.2_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/release.tar.gz"},
		},
	}
	registry, _, _ := newRegistryTestServer(t, nil, map[string]registryRelease{
		"plugin-release/v4.0.2": release,
	})

	_, err := registry.GetDownloadURL("release", "plugin-release/v4.0.2", "Darwin", "arm64")
	if err == nil {
		t.Fatal("GetDownloadURL returned nil error, want missing platform error")
	}
	message := err.Error()
	for _, want := range []string{"release", "plugin-release/v4.0.2", "Darwin/arm64", "plugin-release_4.0.2_Linux_x86_64.tar.gz"} {
		if !strings.Contains(message, want) {
			t.Fatalf("error %q does not contain %q", message, want)
		}
	}
}

func TestRegistryUnknownPluginFailsClearly(t *testing.T) {
	registry, _, _ := newRegistryTestServer(t, nil, nil)

	_, err := registry.GetPluginVersion("deploy")
	if err == nil {
		t.Fatal("GetPluginVersion returned nil error, want unknown plugin error")
	}
	if !strings.Contains(err.Error(), `unknown plugin "deploy"`) {
		t.Fatalf("GetPluginVersion error = %q, want unknown plugin", err)
	}
}

func TestRegistryResolveReleaseTag(t *testing.T) {
	registry, _, _ := newRegistryTestServer(t, [][]registryRelease{
		{{TagName: "plugin-release/v4.0.2"}},
	}, nil)

	tests := []struct {
		version string
		want    string
	}{
		{version: "latest", want: "plugin-release/v4.0.2"},
		{version: "", want: "plugin-release/v4.0.2"},
		{version: "4.0.1", want: "plugin-release/v4.0.1"},
		{version: "v4.0.1", want: "plugin-release/v4.0.1"},
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
}

func newRegistryTestServer(t *testing.T, pages [][]registryRelease, tagReleases map[string]registryRelease) (*Registry, *httptest.Server, *[]string) {
	t.Helper()

	var server *httptest.Server
	paths := []string{}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.String())

		if r.URL.Path == "/releases" {
			page := 1
			if r.URL.Query().Get("page") == "2" {
				page = 2
			}
			if page < len(pages) {
				next := server.URL + "/releases?per_page=100&page=2"
				w.Header().Set("Link", "<"+next+`>; rel="next"`)
			}
			if page > len(pages) {
				writeJSON(t, w, []registryRelease{})
				return
			}
			writeJSON(t, w, pages[page-1])
			return
		}

		if strings.HasPrefix(r.URL.Path, "/releases/tags/") {
			tag, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/releases/tags/"))
			if err != nil {
				t.Fatalf("unescape tag: %v", err)
			}
			release, ok := tagReleases[tag]
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

func newPluginIndexRegistryTestServer(t *testing.T, indexJSON string, tagReleases map[string]registryRelease, pages [][]registryRelease) (*Registry, *httptest.Server, *[]string) {
	t.Helper()

	var server *httptest.Server
	paths := []string{}
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.String())

		if r.URL.Path == "/assets/plugin-index.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(indexJSON))
			return
		}

		if r.URL.Path == "/releases" {
			page := 1
			if r.URL.Query().Get("page") == "2" {
				page = 2
			}
			if page < len(pages) {
				next := server.URL + "/releases?per_page=100&page=2"
				w.Header().Set("Link", "<"+next+`>; rel="next"`)
			}
			if page > len(pages) {
				writeJSON(t, w, []registryRelease{})
				return
			}
			writeJSON(t, w, pages[page-1])
			return
		}

		if strings.HasPrefix(r.URL.Path, "/releases/tags/") {
			tag, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, "/releases/tags/"))
			if err != nil {
				t.Fatalf("unescape tag: %v", err)
			}
			if tag == DefaultPluginIndexReleaseTag {
				writeJSON(t, w, registryRelease{
					TagName: DefaultPluginIndexReleaseTag,
					Assets: []registryAsset{
						{Name: DefaultPluginIndexAssetName, BrowserDownloadURL: server.URL + "/assets/plugin-index.json"},
					},
				})
				return
			}
			release, ok := tagReleases[tag]
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
      "version": "1.0.0",
      "tag": "plugin-ui/v1.0.0",
      "tagPrefix": "plugin-ui/v",
      "manifest": "plugin/ui/manifest.json",
      "assetPrefix": "plugin-ui",
      "binaryName": "plugin-ui",
      "description": "UI component helper plugin"
    },
    {
      "name": "release",
      "unit": "plugin-release",
      "version": "4.0.2",
      "tag": "plugin-release/v4.0.2",
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

func assertNoLatestReleaseCall(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if strings.Contains(path, "/latest") {
			t.Fatalf("registry must not call repository latest release endpoint, called %s", path)
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
