package evidence

import (
	"errors"
	"os"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

func inspectV1CompensationEvidence(path string) ([]EvidenceRecord, []EvidenceDiagnostic, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, []EvidenceDiagnostic{unreadableEvidenceDiagnostic(FamilyV1Compensation, path)}, nil
	}
	var diagnostics []EvidenceDiagnostic
	var evidence release.V1CompensationEvidence
	if !decodeEvidenceJSON(FamilyV1Compensation, path, data, &evidence, &diagnostics) {
		return nil, diagnostics, nil
	}
	if err := evidence.Validate(); err != nil {
		return nil, []EvidenceDiagnostic{invalidEvidenceDiagnostic(FamilyV1Compensation, path)}, nil
	}
	decision := release.SelectV1CompensationOperation(&evidence)
	record := v1CompensationRecord(path, data, evidence, decision)
	return []EvidenceRecord{record}, nil, nil
}

func v1CompensationRecord(path string, data []byte, evidence release.V1CompensationEvidence, decision release.V1CompensationDecision) EvidenceRecord {
	classification, safeToResume, automatic, manual, guidance := classifyV1Compensation(decision)
	return EvidenceRecord{
		Family:                FamilyV1Compensation,
		Identity:              evidence.Identity.SHA256,
		Owner:                 "v1 compensation evidence",
		Version:               evidence.Identity.IntendedVersion,
		Tag:                   evidence.Identity.Tag,
		State:                 string(evidence.Compensation.Status),
		PendingAction:         string(evidence.Compensation.PendingAction),
		Classification:        classification,
		SafeToResume:          safeToResume,
		AutomaticContinuation: automatic,
		ManualRecovery:        manual,
		LifecycleAllowed:      classification == ClassificationCompleted,
		LifecycleOperation:    lifecycleOperation(classification == ClassificationCompleted),
		Guidance:              guidance,
		Path:                  path,
		DigestSHA256:          sha256Hex(data),
		CreatedAt:             formatEvidenceTime(evidence.CreatedAt.String()),
		UpdatedAt:             formatEvidenceTime(evidence.UpdatedAt.String()),
	}
}

func classifyV1Compensation(decision release.V1CompensationDecision) (string, bool, bool, bool, string) {
	switch decision.Kind {
	case release.V1CompensationAlreadyComplete:
		return ClassificationCompleted, false, false, false, "V1 compensation evidence is already completed."
	case release.V1CompensationNoRecoveryNeeded:
		return ClassificationTerminal, false, false, false, "No unsafe V1 release effect is recorded."
	case release.V1CompensationPerformOperation, release.V1CompensationMarkComplete:
		return ClassificationResumable, true, true, false, "V1 compensation has one typed automatic continuation."
	case release.V1CompensationRequireManual:
		return ClassificationManualRecoveryRequired, false, false, true, "V1 compensation requires manual recovery."
	default:
		return ClassificationUncertain, false, false, true, "V1 compensation decision is unknown."
	}
}
