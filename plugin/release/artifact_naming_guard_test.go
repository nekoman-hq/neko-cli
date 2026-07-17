package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	taskLabelPattern      = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9])((?:h|c|dx|f)[1-9][0-9]*(?:[._-][1-9][0-9]*)?)($|[^A-Za-z0-9])`)
	taskIdentifierPattern = regexp.MustCompile(`\b(?:Test|test|assert|write|must|with|read|require|ensure|check|validate|new|build|make|handle|map|parse|load|save|run|compose|create|update|render|format|collect|find|list|inspect|scan)[A-Za-z0-9_]*(?:H|C|DX|F)[1-9][0-9]*(?:[._-][1-9][0-9]*)?[A-Za-z0-9_]*\b`)
	taskSequencePattern   = regexp.MustCompile(`(?i)(^|[^A-Za-z0-9])((?:stage|milestone)[._ -]*[1-9][0-9]*)($|[^A-Za-z0-9])`)
	issueIDPattern        = regexp.MustCompile(`\bNEK-[1-9][0-9]*\b`)
)

func TestTrackedArtifactsDoNotUseTaskManagementCodes(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range trackedFiles(t, root) {
		if !inspectedArtifact(path) {
			continue
		}
		reportPathMatches(t, path)
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for lineNumber, line := range strings.Split(string(data), "\n") {
			reportTaskLabelMatches(t, path, lineNumber+1, line)
			reportTaskIdentifierMatches(t, path, lineNumber+1, line)
			reportTaskSequenceMatches(t, path, lineNumber+1, line)
			if permanentTechnicalDoc(path) {
				reportIssueIDMatches(t, path, lineNumber+1, line)
			}
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func trackedFiles(t *testing.T, root string) []string {
	t.Helper()
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("list tracked files: %v", err)
	}
	parts := bytes.Split(bytes.TrimSuffix(out, []byte{0}), []byte{0})
	paths := make([]string, 0, len(parts))
	for _, part := range parts {
		paths = append(paths, string(part))
	}
	return paths
}

func inspectedArtifact(path string) bool {
	if !textArtifact(path) {
		return false
	}
	return strings.HasPrefix(path, "plugin/release/") ||
		strings.HasPrefix(path, "docs/") ||
		strings.HasPrefix(path, ".github/") ||
		strings.HasPrefix(path, "scripts/") ||
		path == "README.md"
}

func textArtifact(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".md", ".json", ".yaml", ".yml", ".sh":
		return true
	default:
		return false
	}
}

func permanentTechnicalDoc(path string) bool {
	return filepath.Ext(path) == ".md" &&
		(path == "README.md" ||
			strings.HasPrefix(path, "docs/") ||
			strings.HasPrefix(path, "plugin/release/"))
}

func reportPathMatches(t *testing.T, path string) {
	t.Helper()
	for _, match := range taskLabelPattern.FindAllStringSubmatchIndex(path, -1) {
		label := path[match[4]:match[5]]
		if htmlHeadingTag(path, match[4], match[5]) {
			continue
		}
		t.Errorf("%s:0 contains task-management label %q in filename", path, label)
	}
	for _, label := range taskIdentifierPattern.FindAllString(path, -1) {
		t.Errorf("%s:0 contains task-management identifier %q in filename", path, label)
	}
	for _, match := range taskSequencePattern.FindAllStringSubmatchIndex(path, -1) {
		label := path[match[4]:match[5]]
		t.Errorf("%s:0 contains task-management sequence %q in filename", path, label)
	}
}

func reportTaskLabelMatches(t *testing.T, path string, lineNumber int, line string) {
	t.Helper()
	for _, match := range taskLabelPattern.FindAllStringSubmatchIndex(line, -1) {
		label := line[match[4]:match[5]]
		if htmlHeadingTag(line, match[4], match[5]) {
			continue
		}
		t.Errorf("%s:%d contains task-management label %q", path, lineNumber, label)
	}
}

func reportTaskIdentifierMatches(t *testing.T, path string, lineNumber int, line string) {
	t.Helper()
	for _, label := range taskIdentifierPattern.FindAllString(line, -1) {
		t.Errorf("%s:%d contains task-management identifier %q", path, lineNumber, label)
	}
}

func reportTaskSequenceMatches(t *testing.T, path string, lineNumber int, line string) {
	t.Helper()
	for _, match := range taskSequencePattern.FindAllStringSubmatchIndex(line, -1) {
		label := line[match[4]:match[5]]
		t.Errorf("%s:%d contains task-management sequence %q", path, lineNumber, label)
	}
}

func reportIssueIDMatches(t *testing.T, path string, lineNumber int, line string) {
	t.Helper()
	for _, label := range issueIDPattern.FindAllString(line, -1) {
		t.Errorf("%s:%d contains issue-tracker identifier %q", path, lineNumber, label)
	}
}

func htmlHeadingTag(line string, start int, end int) bool {
	label := strings.ToLower(line[start:end])
	if len(label) != 2 || label[0] != 'h' || label[1] < '1' || label[1] > '6' {
		return false
	}
	before := line[:start]
	if !strings.HasSuffix(before, "<") && !strings.HasSuffix(before, "</") {
		return false
	}
	after := line[end:]
	return strings.HasPrefix(after, ">") ||
		strings.HasPrefix(after, " ") ||
		strings.HasPrefix(after, "\t")
}
