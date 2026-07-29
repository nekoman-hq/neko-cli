package pluginindex

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHandlePluginIndexCharacterizesGoErrorBoundary(t *testing.T) {
	t.Run("conflicting modes fail before discovery", func(t *testing.T) {
		root := newPluginIndexTempDir(t)
		t.Chdir(root)

		resp, err := HandlePluginIndex(pluginRequest(map[string]any{
			"check":       true,
			"output-file": "plugin-index.json",
		}))
		if resp != nil || err == nil || !strings.Contains(err.Error(), "--check cannot be used with --output-file") {
			t.Fatalf("HandlePluginIndex = (%#v, %v), want nil response and mode error", resp, err)
		}
	})

	t.Run("discovery failure is a Go error", func(t *testing.T) {
		root := newPluginIndexTempDir(t)
		t.Chdir(root)

		resp, err := HandlePluginIndex(pluginRequest(nil))
		if resp != nil || err == nil || !strings.Contains(err.Error(), "plugin index config invalid") {
			t.Fatalf("HandlePluginIndex = (%#v, %v), want nil response and discovery error", resp, err)
		}
	})
}

func TestHandlePluginIndexCheckContractIsReadOnly(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	before := snapshotIndexTree(t, root)

	resp, err := HandlePluginIndex(pluginRequest(map[string]any{
		"check":      true,
		"pretty":     "false",
		"repository": "example/project",
	}))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	if resp.Status != "success" || resp.Metadata.Command != CommandName || resp.Metadata.Timestamp.IsZero() {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if resp.RendererHint != "" {
		t.Fatalf("renderer = %q, want empty", resp.RendererHint)
	}
	wantItems := []map[string]any{
		{"property": "Status", "value": "ok"},
		{"property": "Plugins", "value": 2},
		{"property": "Repository", "value": "example/project"},
	}
	assertIndexItems(t, resp.Data["items"], wantItems)

	after := snapshotIndexTree(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("check mode mutated repository\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestHandlePluginIndexRawRenderContractIsReadOnly(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	before := snapshotIndexTree(t, root)

	response, err := HandlePluginIndex(pluginRequest(nil))
	if err != nil {
		t.Fatalf("HandlePluginIndex: %v", err)
	}
	if response.RendererHint != "raw-json" {
		t.Fatalf("raw renderer hint = %q", response.RendererHint)
	}
	if after := snapshotIndexTree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("raw render mutated repository\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestGenerateReturnsContextCancellationWithoutMutation(t *testing.T) {
	root := newIndexTestRepo(t)
	before := snapshotIndexTree(t, root)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	index, err := Generate(ctx, GenerateOptions{Root: root})
	if index != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate = (%#v, %v), want context.Canceled", index, err)
	}
	if after := snapshotIndexTree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("canceled generation mutated repository\nbefore: %#v\nafter:  %#v", before, after)
	}
}

func TestHandlePluginIndexOutputPathFormattingOverwriteAndModes(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	outputDir := filepath.Join(newPluginIndexTempDir(t), "nested", "registry")
	output := filepath.Join(outputDir, "plugin-index.json")

	resp, err := HandlePluginIndex(pluginRequest(map[string]any{"output-file": output}))
	if err != nil {
		t.Fatalf("HandlePluginIndex create: %v", err)
	}
	assertIndexItems(t, resp.Data["items"], []map[string]any{
		{"property": "Status", "value": "written"},
		{"property": "Output", "value": output},
		{"property": "Plugins", "value": 2},
		{"property": "Repository", "value": DefaultRepository},
	})
	if resp.RendererHint != "" {
		t.Fatalf("renderer = %q, want empty", resp.RendererHint)
	}
	pretty := readIndexContractFile(t, output)
	if !strings.HasPrefix(pretty, "{\n  \"schemaVersion\": 1,") || !strings.HasSuffix(pretty, "\n") {
		t.Fatalf("unexpected default pretty output: %q", pretty)
	}
	assertIndexMode(t, outputDir, 0755)
	assertIndexMode(t, output, 0644)

	if chmodErr := os.Chmod(output, 0600); chmodErr != nil {
		t.Fatalf("chmod output: %v", chmodErr)
	}
	resp, err = HandlePluginIndex(pluginRequest(map[string]any{
		"output-file": output,
		"pretty":      false,
		"repository":  "example/project",
	}))
	if err != nil {
		t.Fatalf("HandlePluginIndex replace: %v", err)
	}
	compact := readIndexContractFile(t, output)
	if strings.Contains(compact, "\n  ") || strings.Count(compact, "\n") != 1 {
		t.Fatalf("unexpected compact output: %q", compact)
	}
	if !strings.Contains(compact, `"repository":"example/project"`) {
		t.Fatalf("replacement did not use requested repository: %q", compact)
	}
	assertIndexMode(t, output, 0600)
	assertIndexItems(t, resp.Data["items"], []map[string]any{
		{"property": "Status", "value": "written"},
		{"property": "Output", "value": output},
		{"property": "Plugins", "value": 2},
		{"property": "Repository", "value": "example/project"},
	})
}

func TestHandlePluginIndexOutputDirectoryFailurePreservesExistingFile(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	blocker := filepath.Join(newPluginIndexTempDir(t), "blocked")
	const original = "unrelated content"
	if err := os.WriteFile(blocker, []byte(original), 0600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}

	resp, err := HandlePluginIndex(pluginRequest(map[string]any{
		"output-file": filepath.Join(blocker, "plugin-index.json"),
	}))
	if resp != nil || err == nil || !strings.Contains(err.Error(), "plugin index output parent") {
		t.Fatalf("HandlePluginIndex = (%#v, %v), want output parent Go error", resp, err)
	}
	if got := readIndexContractFile(t, blocker); got != original {
		t.Fatalf("failure modified existing file: got %q want %q", got, original)
	}
	assertIndexMode(t, blocker, 0600)
}

func TestHandlePluginIndexInvalidSourceDoesNotCreateOutputOrPartialFile(t *testing.T) {
	root := newIndexTestRepo(t)
	t.Chdir(root)
	writeManifest(t, root, "plugin/release/manifest.json", "release", "9.9.9", "invalid fixture")
	before := snapshotIndexTree(t, root)
	outputRoot := newPluginIndexTempDir(t)
	output := filepath.Join(outputRoot, "nested", "plugin-index.json")

	response, err := HandlePluginIndex(pluginRequest(map[string]any{"output-file": output}))
	if response != nil || err == nil || !strings.Contains(err.Error(), "does not match state version") {
		t.Fatalf("HandlePluginIndex = (%#v, %v), want source validation error", response, err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("invalid source created output: %v", statErr)
	}
	if after := snapshotIndexTree(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("invalid source mutated repository\nbefore: %#v\nafter:  %#v", before, after)
	}
	assertNoPluginIndexTemporaryFiles(t, filepath.Dir(output), output)
}

type indexContractSnapshot struct {
	Data string
	Mode fs.FileMode
}

func snapshotIndexTree(t *testing.T, root string) map[string]indexContractSnapshot {
	t.Helper()
	snapshot := map[string]indexContractSnapshot{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = indexContractSnapshot{Mode: info.Mode().Perm(), Data: string(data)}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

func assertIndexItems(t *testing.T, got any, want []map[string]any) {
	t.Helper()
	items, ok := got.([]map[string]any)
	if !ok || !reflect.DeepEqual(items, want) {
		t.Fatalf("items = %#v, want %#v", got, want)
	}
}

func assertIndexMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode %s = %#o, want %#o", path, got, want)
	}
}

func readIndexContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
