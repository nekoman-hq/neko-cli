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
	}
	if !evidenceFamilyAllowed(request.Family) {
		return evidenceQueryRequest{}, fmt.Errorf("unsupported evidence family %q", request.Family)
	}
	return request, nil
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
