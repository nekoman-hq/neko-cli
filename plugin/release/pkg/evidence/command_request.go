package evidence

import (
	"fmt"
	"strings"
)

const CommandName = "evidence"

type evidenceQueryRequest struct {
	RepositoryRoot string
	Family         string
	Unit           string
	IdentityPrefix string
}

func parseEvidenceQueryRequest(flags map[string]any, workingDir string) (evidenceQueryRequest, error) {
	root := strings.TrimSpace(workingDir)
	if root == "" {
		root = "."
	}
	request := evidenceQueryRequest{
		RepositoryRoot: root,
		Family:         strings.TrimSpace(evidenceFlagString(flags, "family")),
		Unit:           strings.TrimSpace(evidenceFlagString(flags, "unit")),
		IdentityPrefix: strings.TrimSpace(evidenceFlagString(flags, "identity")),
	}
	if !evidenceFamilyAllowed(request.Family) {
		return evidenceQueryRequest{}, fmt.Errorf("unsupported evidence family %q", request.Family)
	}
	if request.IdentityPrefix != "" && !validEvidenceIdentityPrefix(request.IdentityPrefix) {
		return evidenceQueryRequest{}, fmt.Errorf("--identity must contain 8 to 64 lowercase hexadecimal characters")
	}
	return request, nil
}

func validEvidenceIdentityPrefix(identity string) bool {
	if len(identity) < 8 || len(identity) > 64 {
		return false
	}
	for _, character := range identity {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func evidenceFamilyAllowed(family string) bool {
	switch family {
	case "", FamilyReleaseExecution, FamilyDispatch, FamilyMigration, FamilyV1Compensation, FamilyV2PairRecovery:
		return true
	default:
		return false
	}
}

func evidenceFlagString(flags map[string]any, name string) string {
	if value, ok := flags[name].(string); ok {
		return value
	}
	return ""
}
