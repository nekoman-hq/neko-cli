package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func sortedJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read evidence directory %s: %w", dir, err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func readEvidenceBytes(family, path string) ([]byte, EvidenceDiagnostic, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, unreadableEvidenceDiagnostic(family, path), false
	}
	return data, EvidenceDiagnostic{}, true
}

func decodeEvidenceJSON(family, path string, data []byte, target any, diagnostics *[]EvidenceDiagnostic) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		*diagnostics = append(*diagnostics, corruptEvidenceDiagnostic(family, path))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		*diagnostics = append(*diagnostics, corruptEvidenceDiagnostic(family, path))
		return false
	}
	return true
}

func unreadableEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationUncertain,
		Code:           "unreadable",
		Guidance:       "Evidence could not be read. Preserve the file and inspect permissions manually.",
	}
}

func corruptEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationCorrupt,
		Code:           "corrupt-json",
		Guidance:       "Evidence could not be decoded safely. Preserve the file and inspect manually.",
	}
}

func unsupportedEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationUnsupported,
		Code:           "unsupported-schema",
		Guidance:       "Evidence uses an unsupported schema. Preserve the file and inspect with a compatible release plugin.",
	}
}

func invalidEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationCorrupt,
		Code:           "invalid-state",
		Guidance:       "Evidence has invalid typed fields. Preserve the file and recover manually.",
	}
}

func conflictingEvidenceDiagnostic(family, path string) EvidenceDiagnostic {
	return EvidenceDiagnostic{
		Family:         family,
		Path:           path,
		Classification: ClassificationConflicting,
		Code:           "conflicting-identity",
		Guidance:       "Evidence identity or owner paths conflict with its location. Preserve the file and recover manually.",
	}
}

func sortEvidenceResult(result *evidenceQueryResult) {
	sort.SliceStable(result.Records, func(i, j int) bool {
		return evidenceRecordSortKey(result.Records[i]) < evidenceRecordSortKey(result.Records[j])
	})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		return result.Diagnostics[i].Family+"\x00"+result.Diagnostics[i].Path < result.Diagnostics[j].Family+"\x00"+result.Diagnostics[j].Path
	})
}

func evidenceRecordSortKey(record EvidenceRecord) string {
	return record.Family + "\x00" + record.Path + "\x00" + record.Identity
}

func safeEvidenceHash(hash string) bool {
	if len(hash) != 64 {
		return false
	}
	for _, character := range hash {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func lifecycleOperation(allowed bool) string {
	if allowed {
		return "archive-completed"
	}
	return ""
}

func formatEvidenceTime(value string) string {
	if strings.HasPrefix(value, "0001-01-01") {
		return ""
	}
	return value
}
