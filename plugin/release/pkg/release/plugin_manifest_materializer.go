package release

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func appendPluginManifestMaterialization(ctx *ReleaseExecutionContext, plan *MaterializationPlan) error {
	if ctx == nil {
		return fmt.Errorf("release execution context is missing")
	}
	if plan == nil {
		return fmt.Errorf("materialization plan is missing")
	}
	if !ctx.Unit.IsPlugin {
		return nil
	}
	manifestPath := ctx.Unit.PluginManifestPath
	if strings.TrimSpace(manifestPath) == "" {
		return fmt.Errorf("plugin manifest path is missing for unit %s", ctx.Unit.ID)
	}

	change, err := planPluginManifest(ctx, manifestPath)
	if err != nil {
		return err
	}
	if change != nil {
		plan.Changes = append(plan.Changes, *change)
	}
	return nil
}

func planPluginManifest(ctx *ReleaseExecutionContext, manifestPath string) (*MaterializedFileChange, error) {
	path := filepath.Join(ctx.RepositoryRoot, manifestPath)
	before, mode, existed, err := readRequiredPluginManifest(path, manifestPath, ctx.Unit.ID)
	if err != nil {
		return nil, err
	}
	after, err := materializePluginManifest(before, ctx.NextVersion, manifestPath)
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
		"sync plugin manifest version with release plan",
		true,
	)
	if err != nil {
		return nil, err
	}
	return &change, nil
}

func readRequiredPluginManifest(path, displayPath, unitID string) ([]byte, os.FileMode, bool, error) {
	content, mode, existed, err := readMaterializedFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	if !existed {
		return nil, 0, false, fmt.Errorf("required plugin manifest file %s not found for unit %s", displayPath, unitID)
	}
	return content, mode, existed, nil
}

func materializePluginManifest(content []byte, nextVersion, displayPath string) ([]byte, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", displayPath, err)
	}
	versionRaw, ok := doc["version"]
	if !ok {
		return nil, fmt.Errorf("locate version in %s: version not found", displayPath)
	}
	var currentVersion string
	if err := json.Unmarshal(versionRaw, &currentVersion); err != nil {
		return nil, fmt.Errorf("parse version in %s: %w", displayPath, err)
	}

	nextVersionJSON, err := json.Marshal(nextVersion)
	if err != nil {
		return nil, fmt.Errorf("marshal version for %s: %w", displayPath, err)
	}
	return replaceTopLevelJSONStringLine(content, "version", string(nextVersionJSON), displayPath)
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
