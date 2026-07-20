package evidence

import (
	"context"
	"fmt"
	"strings"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/release"
)

type evidenceQueryUseCase struct{}

func newEvidenceQueryUseCase() evidenceQueryUseCase {
	return evidenceQueryUseCase{}
}

func (useCase evidenceQueryUseCase) Query(_ context.Context, request evidenceQueryRequest) (evidenceQueryResult, error) {
	var result evidenceQueryResult
	var locations release.ReleaseEvidenceLocations
	if includesCommonDirectoryEvidence(request.Family) {
		var err error
		locations, err = release.ResolveReleaseEvidenceLocations(request.RepositoryRoot)
		if err != nil {
			return evidenceQueryResult{}, err
		}
	}
	if includeEvidenceFamily(request.Family, FamilyReleaseExecution) {
		records, diagnostics, err := inspectReleaseExecutionJournals(locations.ExecutionJournalDirectory)
		if err != nil {
			return evidenceQueryResult{}, err
		}
		result.Records = appendFilteredEvidenceRecords(result.Records, records, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyDispatch) {
		records, diagnostics, err := inspectDispatchJournals(locations.DispatchJournalDirectory)
		if err != nil {
			return evidenceQueryResult{}, err
		}
		result.Records = appendFilteredEvidenceRecords(result.Records, records, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyMigration) {
		record, diagnostics := inspectMigrationJournal(request.RepositoryRoot)
		result.Records = appendFilteredEvidenceRecords(result.Records, record, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyV1Compensation) {
		record, diagnostics, err := inspectV1CompensationEvidence(locations.V1CompensationPath)
		if err != nil {
			return evidenceQueryResult{}, err
		}
		result.Records = appendFilteredEvidenceRecords(result.Records, record, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	if includeEvidenceFamily(request.Family, FamilyV2PairRecovery) {
		record, diagnostics := inspectV2PairRecoveryEvidence(request.RepositoryRoot)
		result.Records = appendFilteredEvidenceRecords(result.Records, record, request.Unit)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
	}
	sortEvidenceResult(&result)
	if request.IdentityPrefix != "" {
		return selectEvidenceByIdentityPrefix(result, request.IdentityPrefix)
	}
	return result, nil
}

func selectEvidenceByIdentityPrefix(result evidenceQueryResult, prefix string) (evidenceQueryResult, error) {
	matches := make([]EvidenceRecord, 0, 1)
	for _, record := range result.Records {
		if strings.HasPrefix(record.Identity, prefix) {
			matches = append(matches, record)
		}
	}
	switch len(matches) {
	case 0:
		return evidenceQueryResult{}, fmt.Errorf("no evidence identity matches prefix %q after applying family and unit filters", prefix)
	case 1:
		result.Records = matches
		return result, nil
	default:
		return evidenceQueryResult{}, fmt.Errorf("evidence identity prefix %q is ambiguous after applying family and unit filters (%d matches)", prefix, len(matches))
	}
}

func includeEvidenceFamily(selected, family string) bool {
	return selected == "" || selected == family
}

func includesCommonDirectoryEvidence(selected string) bool {
	return includeEvidenceFamily(selected, FamilyReleaseExecution) ||
		includeEvidenceFamily(selected, FamilyDispatch) ||
		includeEvidenceFamily(selected, FamilyV1Compensation)
}

func appendFilteredEvidenceRecords(target, records []EvidenceRecord, unit string) []EvidenceRecord {
	if strings.TrimSpace(unit) == "" {
		return append(target, records...)
	}
	for _, record := range records {
		if record.Unit == unit {
			target = append(target, record)
		}
	}
	return target
}
