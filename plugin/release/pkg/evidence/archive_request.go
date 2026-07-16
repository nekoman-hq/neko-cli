package evidence

import (
	"fmt"
	"strings"
)

const ArchiveCommandName = "evidence-archive"

type evidenceArchiveRequest struct {
	RepositoryRoot string
	Family         string
	Identity       string
	DigestSHA256   string
	ConfirmArchive bool
}

func parseEvidenceArchiveRequest(flags map[string]any, workingDir string) (evidenceArchiveRequest, error) {
	root := strings.TrimSpace(workingDir)
	if root == "" {
		root = "."
	}
	request := evidenceArchiveRequest{
		RepositoryRoot: root,
		Family:         strings.TrimSpace(evidenceFlagString(flags, "family")),
		Identity:       strings.TrimSpace(evidenceFlagString(flags, "identity")),
		DigestSHA256:   strings.TrimSpace(evidenceFlagString(flags, "digest-sha256")),
		ConfirmArchive: evidenceFlagBool(flags, "confirm-archive"),
	}
	if request.Family == "" || !evidenceFamilyAllowed(request.Family) {
		return evidenceArchiveRequest{}, fmt.Errorf("--family must name a supported evidence family")
	}
	if request.Family == FamilyDispatch || request.Family == FamilyMigration {
		return evidenceArchiveRequest{}, fmt.Errorf("evidence family %q has no archival lifecycle operation", request.Family)
	}
	if !safeEvidenceHash(request.Identity) {
		return evidenceArchiveRequest{}, fmt.Errorf("--identity must be a sha256 evidence identity from inspection output")
	}
	if !safeEvidenceHash(request.DigestSHA256) {
		return evidenceArchiveRequest{}, fmt.Errorf("--digest-sha256 must be the current digest from inspection output")
	}
	if !request.ConfirmArchive {
		return evidenceArchiveRequest{}, fmt.Errorf("--confirm-archive is required")
	}
	return request, nil
}

func evidenceFlagBool(flags map[string]any, name string) bool {
	if value, ok := flags[name].(bool); ok {
		return value
	}
	return false
}
