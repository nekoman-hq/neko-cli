package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	pluginReleaseUnitID          = "plugin-release"
	pluginReleaseVersionFilePath = ".plugin.release.neko.json"
	pluginReleaseManifestPath    = "plugin/release/manifest.json"
)

type pluginReleaseVersionsFile struct {
	Plugins map[string]string `json:"plugins"`
}

func appendPluginReleaseMaterialization(ctx *ReleaseExecutionContext, plan *MaterializationPlan) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	if plan == nil {
		return fmt.Errorf("materialization plan is missing")
	}
	if ctx.Unit.ID != pluginReleaseUnitID {
		return nil
	}

	versionFileChange, err := planPluginReleaseVersionFile(ctx)
	if err != nil {
		return err
	}
	if versionFileChange != nil {
		plan.Changes = append(plan.Changes, *versionFileChange)
	}

	manifestChange, err := planPluginReleaseManifest(ctx)
	if err != nil {
		return err
	}
	if manifestChange != nil {
		plan.Changes = append(plan.Changes, *manifestChange)
	}

	return nil
}

func planPluginReleaseVersionFile(ctx *ReleaseExecutionContext) (*MaterializedFileChange, error) {
	path := filepath.Join(ctx.RepositoryRoot, pluginReleaseVersionFilePath)
	before, mode, existed, err := readRequiredPluginReleaseMaterializedFile(path, pluginReleaseVersionFilePath)
	if err != nil {
		return nil, err
	}
	after, err := materializePluginReleaseVersionFile(before, ctx.NextVersion)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(before, after) {
		return nil, nil
	}
	change, err := newMaterializedFileChange(
		ctx,
		path,
		before,
		after,
		mode,
		existed,
		"sync release plugin version map with plugin-release release plan",
		true,
	)
	if err != nil {
		return nil, err
	}
	return &change, nil
}

func planPluginReleaseManifest(ctx *ReleaseExecutionContext) (*MaterializedFileChange, error) {
	path := filepath.Join(ctx.RepositoryRoot, pluginReleaseManifestPath)
	before, mode, existed, err := readRequiredPluginReleaseMaterializedFile(path, pluginReleaseManifestPath)
	if err != nil {
		return nil, err
	}
	after, err := materializePluginReleaseManifest(before, ctx.NextVersion)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(before, after) {
		return nil, nil
	}
	change, err := newMaterializedFileChange(
		ctx,
		path,
		before,
		after,
		mode,
		existed,
		"sync release plugin manifest version with plugin-release release plan",
		true,
	)
	if err != nil {
		return nil, err
	}
	return &change, nil
}

func readRequiredPluginReleaseMaterializedFile(path, displayPath string) ([]byte, os.FileMode, bool, error) {
	content, mode, existed, err := readMaterializedFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	if !existed {
		return nil, 0, false, fmt.Errorf("required plugin-release materialized file %s not found", displayPath)
	}
	return content, mode, existed, nil
}

func materializePluginReleaseVersionFile(content []byte, nextVersion string) ([]byte, error) {
	var doc pluginReleaseVersionsFile
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", pluginReleaseVersionFilePath, err)
	}
	if doc.Plugins == nil {
		return nil, fmt.Errorf("locate plugins in %s: plugins not found", pluginReleaseVersionFilePath)
	}
	if _, ok := doc.Plugins["release"]; !ok {
		return nil, fmt.Errorf("locate plugins.release in %s: release not found", pluginReleaseVersionFilePath)
	}
	doc.Plugins["release"] = nextVersion
	after, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", pluginReleaseVersionFilePath, err)
	}
	return append(after, '\n'), nil
}

func materializePluginReleaseManifest(content []byte, nextVersion string) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", pluginReleaseManifestPath, err)
	}
	versionRaw, ok := doc["version"]
	if !ok {
		return nil, fmt.Errorf("locate version in %s: version not found", pluginReleaseManifestPath)
	}
	var currentVersion string
	if err := json.Unmarshal(versionRaw, &currentVersion); err != nil {
		return nil, fmt.Errorf("parse version in %s: %w", pluginReleaseManifestPath, err)
	}

	nextVersionJSON, err := json.Marshal(nextVersion)
	if err != nil {
		return nil, fmt.Errorf("marshal version for %s: %w", pluginReleaseManifestPath, err)
	}
	return replaceTopLevelJSONStringLine(content, "version", string(nextVersionJSON), pluginReleaseManifestPath)
}

func replaceTopLevelJSONStringLine(content []byte, key, encodedValue, displayPath string) ([]byte, error) {
	lines := strings.SplitAfter(string(content), "\n")
	for i, line := range lines {
		body, lineEnding := splitLineEnding(line)
		trimmed := strings.TrimSpace(body)
		if !strings.HasPrefix(trimmed, `"`+key+`"`) {
			continue
		}
		colonIndex := strings.Index(body, ":")
		if colonIndex < 0 || strings.TrimSpace(body[:colonIndex]) != `"`+key+`"` {
			continue
		}
		comma := ""
		if strings.HasSuffix(trimmed, ",") {
			comma = ","
		}
		indentEnd := strings.Index(body, `"`+key+`"`)
		if indentEnd < 0 {
			return nil, fmt.Errorf("locate %s in %s: malformed key line", key, displayPath)
		}
		lines[i] = body[:indentEnd] + `"` + key + `": ` + encodedValue + comma + lineEnding
		return []byte(strings.Join(lines, "")), nil
	}
	return nil, fmt.Errorf("locate %s in %s: %s line not found", key, displayPath, key)
}

func splitLineEnding(line string) (string, string) {
	if strings.HasSuffix(line, "\r\n") {
		return strings.TrimSuffix(line, "\r\n"), "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return strings.TrimSuffix(line, "\n"), "\n"
	}
	if strings.HasSuffix(line, "\r") {
		return strings.TrimSuffix(line, "\r"), "\r"
	}
	return line, ""
}
