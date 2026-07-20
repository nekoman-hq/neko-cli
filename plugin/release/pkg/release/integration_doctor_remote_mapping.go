package release

import (
	"fmt"
	"strings"
)

func integrationDoctorRemoteFact(
	subject string,
	category string,
	state integrationDoctorVerificationState,
	evidence string,
	workflow string,
	unit string,
	references ...string,
) integrationDoctorVerification {
	fact := newIntegrationDoctorVerification(category, state, evidence, workflow, unit, references...)
	fact.Subject = subject
	switch state {
	case integrationDoctorUnavailable, integrationDoctorUnauthorized, integrationDoctorRateLimited,
		integrationDoctorUnsupported, integrationDoctorNotAttempted, integrationDoctorUnverifiable:
		fact.LimitationClass = integrationDoctorRemoteLimitation
	}
	return fact
}

func integrationDoctorRemoteDiagnostic(
	severity integrationDoctorSeverity,
	code string,
	workflow string,
	unit string,
	message string,
	remediation string,
) *integrationDoctorDiagnostic {
	diagnostic := newIntegrationDoctorDiagnostic(severity, "remote", code, message, remediation)
	diagnostic.Workflow = workflow
	diagnostic.Unit = unit
	return &diagnostic
}

func integrationDoctorMapRemoteRepository(
	identity integrationDoctorRepositoryIdentity,
	repository integrationDoctorGitHubRepository,
	outcome integrationDoctorGitHubReadOutcome,
) (integrationDoctorVerification, *integrationDoctorDiagnostic) {
	fact := integrationDoctorRemoteFact(
		identity.Name(), "remote_workflow_identity", outcome.State,
		integrationDoctorRemoteOutcomeEvidence("GitHub repository identity", outcome),
		"", "", ".git/config",
	)
	if outcome.State != integrationDoctorVerified {
		return fact, integrationDoctorDiagnosticForRemoteOutcome(
			outcome, "REMOTE_REPOSITORY", "", "", "GitHub repository",
		)
	}
	if !strings.EqualFold(repository.Owner, identity.Owner) ||
		!strings.EqualFold(repository.Name, identity.Repository) {
		fact.State = integrationDoctorMismatch
		fact.Evidence = "GitHub returned a repository identity that differs from the locally resolved origin."
		return fact, integrationDoctorRemoteDiagnostic(
			integrationDoctorError, "REMOTE_REPOSITORY_IDENTITY_MISMATCH", "", "",
			"The remote repository identity does not match the locally resolved origin.",
			"Use one exact GitHub repository identity for local origin and Release V2 integration.",
		)
	}
	visibility := repository.Visibility
	if visibility == "" {
		if repository.Private {
			visibility = "private"
		} else {
			visibility = "public"
		}
	}
	fact.Evidence = fmt.Sprintf(
		"GitHub repository %s exists with default branch %s and %s visibility.",
		identity.Name(), repository.DefaultBranch, visibility,
	)
	return fact, nil
}

func integrationDoctorDiagnosticForRemoteOutcome(
	outcome integrationDoctorGitHubReadOutcome,
	codePrefix string,
	workflow string,
	unit string,
	subject string,
) *integrationDoctorDiagnostic {
	switch outcome.State {
	case integrationDoctorMissing:
		return integrationDoctorRemoteDiagnostic(
			integrationDoctorError, codePrefix+"_MISSING", workflow, unit,
			fmt.Sprintf("The exact %s is missing remotely.", subject),
			fmt.Sprintf("Create or restore the exact %s required by the inspected local contract.", subject),
		)
	case integrationDoctorUnauthorized:
		return integrationDoctorRemoteDiagnostic(
			integrationDoctorNotVerifiable, codePrefix+"_UNAUTHORIZED", workflow, unit,
			fmt.Sprintf("The exact %s could not be verified with the available identity.", subject),
			"Provide a read-capable token only when protected metadata verification is required.",
		)
	case integrationDoctorRateLimited:
		return integrationDoctorRemoteDiagnostic(
			integrationDoctorNotVerifiable, codePrefix+"_RATE_LIMITED", workflow, unit,
			fmt.Sprintf("The exact %s could not be verified because GitHub rate-limited the GET request.%s", subject, integrationDoctorRemoteRateLimitSuffix(outcome)),
			"Repeat the explicit read-only verification after the reported limit resets; no automatic retry was attempted.",
		)
	case integrationDoctorUnsupported:
		return integrationDoctorRemoteDiagnostic(
			integrationDoctorNotVerifiable, codePrefix+"_UNSUPPORTED", workflow, unit,
			fmt.Sprintf("The exact %s returned a remote shape or state that this Doctor does not claim to understand.", subject),
			"Inspect the exact GitHub metadata without mutating the repository.",
		)
	case integrationDoctorNotAttempted:
		return nil
	default:
		return integrationDoctorRemoteDiagnostic(
			integrationDoctorNotVerifiable, codePrefix+"_UNAVAILABLE", workflow, unit,
			fmt.Sprintf("The exact %s was unavailable to the explicit GET-only verification.", subject),
			"Retry only through a new explicit remote verification after transport or service availability is restored.",
		)
	}
}

func integrationDoctorRemoteOutcomeEvidence(
	subject string,
	outcome integrationDoctorGitHubReadOutcome,
) string {
	switch outcome.State {
	case integrationDoctorMissing:
		return subject + " is definitely missing for the authenticated or public exact lookup."
	case integrationDoctorUnauthorized:
		return subject + " could not be resolved with the available identity; an unauthenticated not-found result was not treated as proof of absence."
	case integrationDoctorRateLimited:
		return subject + " was rate limited." + integrationDoctorRemoteRateLimitSuffix(outcome)
	case integrationDoctorUnavailable:
		return subject + " was unavailable through the bounded GET request."
	case integrationDoctorUnsupported:
		return subject + " returned an unsupported state or response shape."
	case integrationDoctorNotAttempted:
		return subject + " was not attempted because an exact prerequisite was unavailable."
	default:
		return subject + " was verified through an exact GET request."
	}
}

func integrationDoctorRemoteRateLimitSuffix(outcome integrationDoctorGitHubReadOutcome) string {
	parts := make([]string, 0, 2)
	if outcome.RetryAfter != "" {
		parts = append(parts, "retry after "+outcome.RetryAfter)
	}
	if outcome.RateLimitReset != "" {
		parts = append(parts, "reset at "+outcome.RateLimitReset)
	}
	if len(parts) == 0 {
		return ""
	}
	return " Safe rate-limit metadata: " + strings.Join(parts, ", ") + "."
}
