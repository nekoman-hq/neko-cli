package doctor

import (
	"context"

	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

// VerificationState is the neutral outcome of one Doctor-owned verification
// fact. It intentionally excludes diagnostics, readiness, remediation, and
// presentation policy.
type VerificationState string

const (
	VerificationVerified     VerificationState = "verified"
	VerificationMissing      VerificationState = "missing"
	VerificationMismatch     VerificationState = "mismatch"
	VerificationNotAttempted VerificationState = "not_attempted"
	VerificationUnavailable  VerificationState = "unavailable"
	VerificationUnauthorized VerificationState = "unauthorized"
	VerificationRateLimited  VerificationState = "rate_limited"
	VerificationUnsupported  VerificationState = "unsupported"
	VerificationUnverifiable VerificationState = "not_verifiable"
)

// VerificationLimitation identifies why a fact cannot be verified by the
// inspection boundary that produced it.
type VerificationLimitation string

const (
	VerificationRemoteLimitation           VerificationLimitation = "remote"
	VerificationRuntimeLimitation          VerificationLimitation = "runtime"
	VerificationMutationRequiredLimitation VerificationLimitation = "mutation_required"
)

// VerificationFact is the immutable neutral fact shared with read-only
// inspection consumers. References remain repository-relative and values never
// include credentials or secret contents.
//
//nolint:govet // Field order follows fact identity and evidence order.
type VerificationFact struct {
	Subject         string
	Category        string
	State           VerificationState
	Evidence        string
	References      []string
	Unit            string
	Workflow        string
	LimitationClass VerificationLimitation
	Remote          bool
}

// VerificationRemoteSummary reports whether the Doctor's existing focused
// remote read boundary participated in a snapshot.
type VerificationRemoteSummary struct {
	Status     string
	Requested  bool
	Verified   int
	Unresolved int
	Failed     int
}

// VerificationSnapshot contains only neutral facts and their remote-read
// summary. It deliberately cannot expose Doctor diagnostics or presentation.
type VerificationSnapshot struct {
	Facts  []VerificationFact
	Remote VerificationRemoteSummary
}

// InspectLocalVerification returns Doctor-owned local verification facts. It
// constructs no remote client, resolves no token, and performs no network I/O.
func InspectLocalVerification(
	ctx context.Context,
	root workspace.RepositoryRoot,
	unitID string,
) VerificationSnapshot {
	result := (integrationDoctorInspectionUseCase{
		sources:    filesystemIntegrationDoctorSourceReader{},
		workflows:  filesystemIntegrationDoctorWorkflowReader{},
		files:      filesystemIntegrationDoctorRepositoryFileReader{},
		identities: filesystemIntegrationDoctorRepositoryIdentityReader{},
	}).Inspect(ctx, integrationDoctorRequest{
		RepositoryRoot: root.Path(),
		UnitID:         unitID,
	})
	return pipelineVerificationSnapshot(result)
}

func pipelineVerificationSnapshot(result *integrationDoctorResult) VerificationSnapshot {
	if result == nil {
		return VerificationSnapshot{Facts: make([]VerificationFact, 0)}
	}
	facts := make([]VerificationFact, 0, len(result.Verifications))
	for _, fact := range result.Verifications {
		facts = append(facts, VerificationFact{
			Subject: fact.Subject, Category: fact.Category,
			State: VerificationState(fact.State), Evidence: fact.Evidence,
			References: append([]string(nil), fact.References...),
			Unit:       fact.Unit, Workflow: fact.Workflow,
			LimitationClass: VerificationLimitation(fact.LimitationClass),
			Remote:          fact.Remote,
		})
	}
	return VerificationSnapshot{
		Facts: facts,
		Remote: VerificationRemoteSummary{
			Status: string(result.RemoteVerification.Status), Requested: result.RemoteVerification.Requested,
			Verified: result.RemoteVerification.Verified, Unresolved: result.RemoteVerification.Unresolved,
			Failed: result.RemoteVerification.Failed,
		},
	}
}
