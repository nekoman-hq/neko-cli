package release

func applyIntegrationDoctorRemoteInspection(
	result *integrationDoctorResult,
	remote integrationDoctorRemoteInspection,
) {
	if result == nil {
		return
	}
	result.RemoteVerification = remote.Summary
	result.Verifications = integrationDoctorWithoutOfflineRemoteFacts(result.Verifications)
	result.Diagnostics = integrationDoctorWithoutOfflineRemoteDiagnostics(result.Diagnostics)
	result.Verifications = append(result.Verifications, remote.Verifications...)
	result.Diagnostics = append(result.Diagnostics, remote.Diagnostics...)
	integrationDoctorNarrowResidualLimitations(result, remote)
}

func integrationDoctorWithoutOfflineRemoteFacts(
	facts []integrationDoctorVerification,
) []integrationDoctorVerification {
	filtered := make([]integrationDoctorVerification, 0, len(facts))
	for _, fact := range facts {
		if fact.Category == "remote_workflow_identity" || fact.Category == "repository_variable_values" {
			continue
		}
		filtered = append(filtered, fact)
	}
	return filtered
}

func integrationDoctorWithoutOfflineRemoteDiagnostics(
	diagnostics []integrationDoctorDiagnostic,
) []integrationDoctorDiagnostic {
	filtered := make([]integrationDoctorDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		switch diagnostic.Code {
		case "REMOTE_WORKFLOW_NOT_VERIFIABLE", "REPOSITORY_VARIABLES_NOT_VERIFIABLE":
			continue
		}
		filtered = append(filtered, diagnostic)
	}
	return filtered
}

func integrationDoctorNarrowResidualLimitations(
	result *integrationDoctorResult,
	remote integrationDoctorRemoteInspection,
) {
	installation := integrationDoctorRemoteCategoryStatus(remote.Verifications, "installation_wiring")
	credentials := integrationDoctorRemoteCategoryStatus(remote.Verifications, "credential_wiring")
	publication := integrationDoctorRemoteCategoryStatus(remote.Verifications, "publication_identity")
	filtered := make([]integrationDoctorDiagnostic, 0, len(result.Diagnostics))
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Code {
		case "INSTALLATION_ARTIFACTS_NOT_VERIFIABLE":
			if installation.failed {
				continue
			}
			if installation.complete {
				diagnostic.Message = "Exact pinned CLI and Release Plugin releases and required assets were remotely verified; download, extraction, execution, and plugin loading remain runtime uncertainties."
				diagnostic.Remediation = "Exercise the exact verified artifacts in a controlled runtime and retain installation evidence."
			} else {
				diagnostic.Message = "Local installation identity remains verified, but one or more explicit remote availability checks were unresolved; download, extraction, execution, and plugin loading also remain runtime uncertainties."
			}
		case "PUBLICATION_CREDENTIALS_NOT_VERIFIABLE":
			if credentials.complete {
				diagnostic.Message = "Local credential wiring and every referenced custom secret name were verified without reading secret values; runtime issuance, value validity, expiry, authorization, and service acceptance remain unknown."
			} else if credentials.observed {
				diagnostic.Message = "Local credential wiring remains verified, but some referenced custom secret-name metadata was unresolved; runtime issuance, value validity, expiry, authorization, and service acceptance remain unknown."
			}
		case "PUBLICATION_TARGET_NOT_VERIFIABLE":
			if publication.complete {
				diagnostic.Message = "The exact repository, current tags, releases, and locally derived artifact identities were remotely observed; acceptance of a future version, uploads, overwrites, and service availability at publication time remain unknown."
				diagnostic.Remediation = "Treat the next publication as a separate mutation with its existing journal and recovery boundaries."
			} else if publication.observed {
				diagnostic.Message = "Local publication identity remains verified, but some exact remote target-state reads were unresolved or failed; future publication acceptance and runtime service behavior remain unknown."
			}
		}
		filtered = append(filtered, diagnostic)
	}
	result.Diagnostics = filtered
}

type integrationDoctorRemoteCategoryObservation struct {
	observed bool
	complete bool
	failed   bool
}

func integrationDoctorRemoteCategoryStatus(
	facts []integrationDoctorVerification,
	category string,
) integrationDoctorRemoteCategoryObservation {
	observation := integrationDoctorRemoteCategoryObservation{complete: true}
	for _, fact := range facts {
		if fact.Category != category {
			continue
		}
		observation.observed = true
		switch fact.State {
		case integrationDoctorVerified:
		case integrationDoctorMissing, integrationDoctorMismatch:
			observation.failed = true
			observation.complete = false
		default:
			observation.complete = false
		}
	}
	if !observation.observed {
		observation.complete = false
	}
	return observation
}
