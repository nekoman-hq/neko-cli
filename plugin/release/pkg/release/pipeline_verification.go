package release

import (
	"context"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/doctor"
	"github.com/nekoman-hq/neko-cli/plugin/release/internal/pipelineinspection"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/workspace"
)

func inspectPipelineVerification(
	root workspace.RepositoryRoot,
	request plugin.Request,
) (pipelineinspection.VerificationSnapshot, error) {
	unitID, _ := request.Flags["unit"].(string)
	remoteRequested := pipelineinspection.RequestsRemoteVerification(request)
	var snapshot doctor.VerificationSnapshot
	if remoteRequested {
		var err error
		snapshot, err = doctor.InspectRemoteVerification(context.Background(), root, unitID)
		if err != nil {
			return pipelineinspection.VerificationSnapshot{}, err
		}
	} else {
		snapshot = doctor.InspectLocalVerification(context.Background(), root, unitID)
	}
	facts := make([]pipelineinspection.VerificationFact, 0, len(snapshot.Facts))
	for _, fact := range snapshot.Facts {
		facts = append(facts, pipelineVerificationFact(fact, snapshot.Remote.Requested))
	}
	return pipelineinspection.VerificationSnapshot{
		Facts: facts, RemoteStatus: snapshot.Remote.Status,
		RemoteRequested: snapshot.Remote.Requested,
		RemoteAttempted: pipelineRemoteVerificationAttempted(snapshot),
	}, nil
}

func pipelineVerificationFact(
	fact doctor.VerificationFact,
	remoteRequested bool,
) pipelineinspection.VerificationFact {
	class := pipelineVerificationClass(fact.LimitationClass)
	if fact.Remote {
		class = pipelineinspection.VerificationRemote
	}
	return pipelineinspection.VerificationFact{
		Category: fact.Category, Class: class,
		Status:  pipelineVerificationStatus(fact.State, class, remoteRequested),
		Subject: fact.Subject, Evidence: fact.Evidence,
		Source: "doctor", Scope: pipelineVerificationScope(fact),
		References: append([]string(nil), fact.References...),
		Unit:       fact.Unit, Workflow: fact.Workflow,
	}
}

func pipelineRemoteVerificationAttempted(snapshot doctor.VerificationSnapshot) bool {
	if !snapshot.Remote.Requested {
		return false
	}
	for _, fact := range snapshot.Facts {
		if fact.Remote {
			return true
		}
	}
	return false
}

func pipelineVerificationClass(limitation doctor.VerificationLimitation) pipelineinspection.VerificationClass {
	switch limitation {
	case doctor.VerificationRemoteLimitation:
		return pipelineinspection.VerificationRemote
	case doctor.VerificationRuntimeLimitation:
		return pipelineinspection.VerificationRuntimeRequired
	case doctor.VerificationMutationRequiredLimitation:
		return pipelineinspection.VerificationMutationRequired
	default:
		return pipelineinspection.VerificationLocal
	}
}

func pipelineVerificationStatus(
	state doctor.VerificationState,
	class pipelineinspection.VerificationClass,
	remoteRequested bool,
) pipelineinspection.VerificationStatus {
	switch state {
	case doctor.VerificationVerified:
		return pipelineinspection.VerificationVerified
	case doctor.VerificationMissing, doctor.VerificationMismatch:
		return pipelineinspection.VerificationFailed
	case doctor.VerificationUnavailable:
		return pipelineinspection.VerificationUnavailable
	case doctor.VerificationUnauthorized:
		return pipelineinspection.VerificationUnauthorized
	case doctor.VerificationRateLimited:
		return pipelineinspection.VerificationRateLimited
	case doctor.VerificationNotAttempted:
		return pipelineinspection.VerificationNotChecked
	case doctor.VerificationUnverifiable:
		if class == pipelineinspection.VerificationRemote && !remoteRequested {
			return pipelineinspection.VerificationNotChecked
		}
		return pipelineinspection.VerificationUnresolved
	case doctor.VerificationUnsupported:
		return pipelineinspection.VerificationUnresolved
	default:
		return pipelineinspection.VerificationUnresolved
	}
}

func pipelineVerificationScope(fact doctor.VerificationFact) string {
	switch {
	case fact.Unit != "":
		return "unit"
	case fact.Workflow != "":
		return "workflow"
	default:
		return "repository"
	}
}
