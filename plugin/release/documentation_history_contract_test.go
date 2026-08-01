package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

const releaseHistoryDirectory = "plugin/release/docs/history"

var (
	historyFilenamePattern = regexp.MustCompile(`^[0-9]{3}-[a-z0-9]+(?:-[a-z0-9]+)*\.md$`)
	historyDatePattern     = regexp.MustCompile(`^(?:[0-9]{4}-[0-9]{2}-[0-9]{2}|Unknown|Approximate: [0-9]{4}(?:-[0-9]{2})?)$`)
	markdownLinkPattern    = regexp.MustCompile(`\[[^]]*\]\(([^)]+)\)`)
	markdownHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*#*[ \t]*$`)
)

func TestReleaseDocumentationHistoryContract(t *testing.T) {
	root := repositoryRoot(t)
	historyRoot := filepath.Join(root, filepath.FromSlash(releaseHistoryDirectory))
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		t.Fatalf("read release documentation history: %v", err)
	}

	index := readDocumentationFile(t, filepath.Join(historyRoot, "README.md"))
	entryNames := make([]string, 0, len(entries))
	sequenceOwners := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "README.md" {
			continue
		}
		name := entry.Name()
		if !historyFilenamePattern.MatchString(name) {
			t.Errorf("history entry filename %q does not use a three-digit descriptive sequence", name)
			continue
		}
		sequence := name[:3]
		if owner, exists := sequenceOwners[sequence]; exists {
			t.Errorf("history sequence %s is used by both %s and %s", sequence, owner, name)
		}
		sequenceOwners[sequence] = name
		entryNames = append(entryNames, name)
	}
	sort.Strings(entryNames)
	if len(entryNames) == 0 {
		t.Fatal("release documentation history contains no numbered entries")
	}

	metadataByEntry := make(map[string]map[string]string, len(entryNames))
	lastIndexPosition := -1
	for _, name := range entryNames {
		if count := strings.Count(index, "]("+name+")"); count != 1 {
			t.Errorf("history index references %s %d times, want exactly once", name, count)
		}
		position := strings.Index(index, "]("+name+")")
		if position <= lastIndexPosition {
			t.Errorf("history index is not ordered by permanent sequence at %s", name)
		}
		lastIndexPosition = position
		metadataByEntry[name] = assertHistoryEntryMetadata(t, historyRoot, name)
	}
	assertHistoryRelationships(t, metadataByEntry)

	for _, required := range []string{
		"## Purpose", "## Numbering policy", "## Chronological index",
		"## Evolution summary", "## Current sources", "## Adding a new entry",
	} {
		if !strings.Contains(index, required) {
			t.Errorf("history index is missing %q", required)
		}
	}
	for path, classification := range map[string]string{
		"plugin/release/docs/architecture/refactor-plan.md":           "merge-duplicate",
		"plugin/release/docs/architecture/refactor-history.md":        "move",
		"plugin/release/docs/architecture/post-refactor-review.md":    "split",
		"plugin/release/docs/architecture/post-refactor-roadmap.md":   "merge-duplicate",
		"plugin/release/docs/architecture/architecture-evolution.md":  "split",
		"plugin/release/docs/architecture/current-state.md":           "retain-current",
		"plugin/release/docs/architecture/maintainability-policy.md":  "retain-current",
		"plugin/release/docs/architecture/compatibility-notes.md":     "retain-current",
		"plugin/release/docs/architecture/v1-compatibility-policy.md": "retain-current",
		"docs/release/bootstrap-product-boundary.md":                  "retain-current",
		"docs/release/compatibility.md":                               "retain-current",
		"docs/release/github-actions-golden-path.md":                  "exclude-nonhistorical",
		"docs/plugins/plugin-deploy.md":                               "exclude-nonhistorical",
		"docs/plugins/plugin-monitoring.md":                           "exclude-nonhistorical",
	} {
		linePattern := regexp.MustCompile(`(?m)^\| ` + regexp.QuoteMeta("`"+path+"`") + ` \|.*\| ` + regexp.QuoteMeta(classification) + ` \|$`)
		if !linePattern.MatchString(index) {
			t.Errorf("history inventory is missing %s with classification %s", path, classification)
		}
	}
}

func assertHistoryEntryMetadata(t *testing.T, historyRoot, name string) map[string]string {
	t.Helper()
	content := readDocumentationFile(t, filepath.Join(historyRoot, name))
	metadata := parseHistoryMetadata(content)
	for _, field := range []string{
		"Sequence", "Title", "Status", "Created", "Completed or superseded",
		"Predecessor", "Successor", "Current references", "Original source",
	} {
		if strings.TrimSpace(metadata[field]) == "" {
			t.Errorf("history entry %s is missing metadata field %q", name, field)
		}
	}
	if sequence := metadata["Sequence"]; sequence != name[:3] {
		t.Errorf("history entry %s sequence = %q, want %s", name, sequence, name[:3])
	}
	allowedStatus := map[string]bool{
		"proposed": true, "active": true, "completed": true,
		"superseded": true, "abandoned": true,
	}
	if !allowedStatus[metadata["Status"]] {
		t.Errorf("history entry %s status = %q, outside the closed vocabulary", name, metadata["Status"])
	}
	for _, field := range []string{"Created", "Completed or superseded"} {
		if !historyDatePattern.MatchString(metadata[field]) {
			t.Errorf("history entry %s metadata %s = %q, want ISO date or an explicit unknown/approximate value", name, field, metadata[field])
		}
	}
	if !strings.Contains(content, "This is a historical record") ||
		!strings.Contains(content, "not the current") {
		t.Errorf("history entry %s lacks an explicit non-authoritative historical notice", name)
	}
	currentReferences := markdownTargets(metadata["Current references"])
	if len(currentReferences) == 0 {
		t.Errorf("history entry %s has no linked current reference", name)
	}
	for _, target := range currentReferences {
		resolved := resolveMarkdownTarget(filepath.Join(historyRoot, name), target)
		if pathWithin(resolved, historyRoot) {
			t.Errorf("history entry %s current reference %q points back into history", name, target)
		}
	}
	for lineNumber, line := range strings.Split(content, "\n") {
		reportTaskLabelMatches(t, filepath.ToSlash(filepath.Join(releaseHistoryDirectory, name)), lineNumber+1, line)
		reportTaskIdentifierMatches(t, filepath.ToSlash(filepath.Join(releaseHistoryDirectory, name)), lineNumber+1, line)
		reportTaskSequenceMatches(t, filepath.ToSlash(filepath.Join(releaseHistoryDirectory, name)), lineNumber+1, line)
		reportIssueIDMatches(t, filepath.ToSlash(filepath.Join(releaseHistoryDirectory, name)), lineNumber+1, line)
	}
	return metadata
}

func assertHistoryRelationships(t *testing.T, metadataByEntry map[string]map[string]string) {
	t.Helper()
	for name, metadata := range metadataByEntry {
		for relation, reciprocal := range map[string]string{"Predecessor": "Successor", "Successor": "Predecessor"} {
			value := metadata[relation]
			if value == "None" {
				continue
			}
			targets := markdownTargets(value)
			if len(targets) != 1 {
				t.Errorf("history entry %s %s must be None or exactly one relative link", name, relation)
				continue
			}
			targetName := filepath.Base(strings.Split(targets[0], "#")[0])
			targetMetadata, exists := metadataByEntry[targetName]
			if !exists {
				t.Errorf("history entry %s %s points outside the numbered series: %s", name, relation, targets[0])
				continue
			}
			if !strings.Contains(targetMetadata[reciprocal], name) {
				t.Errorf("history relationship %s -> %s is not reciprocal through %s", name, targetName, reciprocal)
			}
		}
	}
}

func TestReleaseDocumentationHasNoDuplicateActiveRoadmap(t *testing.T) {
	root := repositoryRoot(t)
	for _, path := range []string{
		"plugin/release/docs/architecture/refactor-plan.md",
		"plugin/release/docs/architecture/refactor-history.md",
		"plugin/release/docs/architecture/post-refactor-review.md",
		"plugin/release/docs/architecture/post-refactor-roadmap.md",
		"plugin/release/docs/architecture/architecture-evolution.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("historical roadmap source remains active at %s", path)
		}
	}
}

func TestRepositoryInternalMarkdownLinksResolve(t *testing.T) {
	root := repositoryRoot(t)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, target := range markdownTargets(string(content)) {
			assertMarkdownTargetResolves(t, root, path, target)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository Markdown: %v", err)
	}
}

func parseHistoryMetadata(content string) map[string]string {
	metadata := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "> **") {
			continue
		}
		field, value, found := strings.Cut(strings.TrimPrefix(line, "> **"), ":** ")
		if found {
			metadata[field] = value
		}
	}
	return metadata
}

func markdownTargets(content string) []string {
	matches := markdownLinkPattern.FindAllStringSubmatch(content, -1)
	targets := make([]string, 0, len(matches))
	for _, match := range matches {
		target := strings.TrimSpace(match[1])
		if strings.HasPrefix(target, "<") && strings.HasSuffix(target, ">") {
			target = strings.TrimSuffix(strings.TrimPrefix(target, "<"), ">")
		}
		if destination, _, found := strings.Cut(target, ` "`); found {
			target = destination
		}
		targets = append(targets, target)
	}
	return targets
}

func assertMarkdownTargetResolves(t *testing.T, root, source, target string) {
	t.Helper()
	if target == "" || strings.HasPrefix(target, "/") {
		return
	}
	parsed, err := url.Parse(target)
	if err != nil {
		t.Errorf("%s has invalid Markdown target %q: %v", relativeDocPath(root, source), target, err)
		return
	}
	if parsed.Scheme != "" {
		return
	}
	pathPart, fragment, _ := strings.Cut(target, "#")
	decodedPath, err := url.PathUnescape(pathPart)
	if err != nil {
		t.Errorf("%s has invalid escaped Markdown target %q: %v", relativeDocPath(root, source), target, err)
		return
	}
	resolved := source
	if decodedPath != "" {
		resolved = filepath.Clean(filepath.Join(filepath.Dir(source), filepath.FromSlash(decodedPath)))
	}
	info, err := os.Stat(resolved)
	if err != nil {
		t.Errorf("%s has unresolved internal link %q", relativeDocPath(root, source), target)
		return
	}
	if info.IsDir() {
		resolved = filepath.Join(resolved, "README.md")
		if _, err := os.Stat(resolved); err != nil {
			t.Errorf("%s links to directory %q without README.md", relativeDocPath(root, source), target)
			return
		}
	}
	if fragment != "" && !documentHasAnchor(readDocumentationFile(t, resolved), fragment) {
		t.Errorf("%s has unresolved heading fragment %q in link %q", relativeDocPath(root, source), fragment, target)
	}
}

func documentHasAnchor(content, fragment string) bool {
	decoded, err := url.PathUnescape(fragment)
	if err != nil {
		return false
	}
	wanted := strings.ToLower(decoded)
	if strings.Contains(content, `id="`+wanted+`"`) || strings.Contains(content, `name="`+wanted+`"`) {
		return true
	}
	seen := make(map[string]int)
	for _, match := range markdownHeadingPattern.FindAllStringSubmatch(content, -1) {
		base := githubHeadingSlug(match[1])
		count := seen[base]
		seen[base] = count + 1
		slug := base
		if count > 0 {
			slug = fmt.Sprintf("%s-%d", base, count)
		}
		if slug == wanted {
			return true
		}
	}
	return false
}

func githubHeadingSlug(heading string) string {
	heading = strings.ToLower(strings.TrimSpace(heading))
	slug := make([]rune, 0, len(heading))
	for _, character := range heading {
		switch {
		case unicode.IsLetter(character), unicode.IsDigit(character), character == '-', character == '_':
			slug = append(slug, character)
		case unicode.IsSpace(character):
			slug = append(slug, '-')
		}
	}
	return string(slug)
}

func resolveMarkdownTarget(source, target string) string {
	pathPart, _, _ := strings.Cut(target, "#")
	decodedPath, err := url.PathUnescape(pathPart)
	if err != nil {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), filepath.FromSlash(decodedPath)))
}

func pathWithin(path, directory string) bool {
	relative, err := filepath.Rel(directory, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func readDocumentationFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read documentation file %s: %v", path, err)
	}
	return string(content)
}

func relativeDocPath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}
