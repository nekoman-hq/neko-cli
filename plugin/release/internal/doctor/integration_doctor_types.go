package doctor

import (
	"fmt"
	"sort"
)

const integrationDoctorCommandName = "doctor"

type integrationDoctorSeverity string

const (
	integrationDoctorError          integrationDoctorSeverity = "error"
	integrationDoctorWarning        integrationDoctorSeverity = "warning"
	integrationDoctorRecommendation integrationDoctorSeverity = "recommendation"
	integrationDoctorNotVerifiable  integrationDoctorSeverity = "not_verifiable"
)

type integrationDoctorReadiness string

const (
	integrationDoctorReady             integrationDoctorReadiness = "ready"
	integrationDoctorReadyWithWarnings integrationDoctorReadiness = "ready_with_warnings"
	integrationDoctorNotReady          integrationDoctorReadiness = "not_ready"
)

type integrationDoctorRequest struct {
	RepositoryRoot string
	UnitID         string
	VerifyRemote   bool
}

type integrationDoctorSummary struct {
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Recommendations int `json:"recommendations"`
	NotVerifiable   int `json:"not_verifiable"`
	Verified        int `json:"verified"`
}

type integrationDoctorVerificationState string

const (
	integrationDoctorVerified     integrationDoctorVerificationState = "verified"
	integrationDoctorMissing      integrationDoctorVerificationState = "missing"
	integrationDoctorMismatch     integrationDoctorVerificationState = "mismatch"
	integrationDoctorNotAttempted integrationDoctorVerificationState = "not_attempted"
	integrationDoctorUnavailable  integrationDoctorVerificationState = "unavailable"
	integrationDoctorUnauthorized integrationDoctorVerificationState = "unauthorized"
	integrationDoctorRateLimited  integrationDoctorVerificationState = "rate_limited"
	integrationDoctorUnsupported  integrationDoctorVerificationState = "unsupported"
	integrationDoctorUnverifiable integrationDoctorVerificationState = "not_verifiable"
)

type integrationDoctorRemoteStatus string

const (
	integrationDoctorRemoteNotRequested integrationDoctorRemoteStatus = "not_requested"
	integrationDoctorRemoteComplete     integrationDoctorRemoteStatus = "complete"
	integrationDoctorRemotePartial      integrationDoctorRemoteStatus = "partial"
	integrationDoctorRemoteUnavailable  integrationDoctorRemoteStatus = "unavailable"
)

type integrationDoctorRemoteSummary struct {
	Status     integrationDoctorRemoteStatus `json:"status"`
	Requested  bool                          `json:"requested"`
	Verified   int                           `json:"verified"`
	Unresolved int                           `json:"unresolved"`
	Failed     int                           `json:"failed"`
}

type integrationDoctorLimitationClass string

const (
	integrationDoctorRemoteLimitation           integrationDoctorLimitationClass = "remote"
	integrationDoctorRuntimeLimitation          integrationDoctorLimitationClass = "runtime"
	integrationDoctorMutationRequiredLimitation integrationDoctorLimitationClass = "mutation_required"
)

//nolint:govet // Field order preserves the additive JSON contract.
type integrationDoctorVerification struct {
	Subject         string                             `json:"subject"`
	Category        string                             `json:"category"`
	State           integrationDoctorVerificationState `json:"state"`
	Evidence        string                             `json:"evidence"`
	References      []string                           `json:"references"`
	Unit            string                             `json:"unit,omitempty"`
	Workflow        string                             `json:"workflow,omitempty"`
	LimitationClass integrationDoctorLimitationClass   `json:"limitation_class,omitempty"`
	Remote          bool                               `json:"-"`
}

type integrationDoctorUnit struct {
	ID               string `json:"id"`
	Version          string `json:"version"`
	TagPrefix        string `json:"tag_prefix"`
	Executor         string `json:"executor"`
	Delivery         string `json:"delivery"`
	Workflow         string `json:"workflow"`
	WorkingDirectory string `json:"working_directory"`
}

//nolint:govet // Field order preserves the stable JSON contract.
type integrationDoctorWorkflow struct {
	Path           string   `json:"path"`
	Units          []string `json:"units"`
	Classification string   `json:"classification"`
	Exists         bool     `json:"exists"`
}

type integrationDoctorDiagnostic struct {
	Severity    integrationDoctorSeverity `json:"severity"`
	Scope       string                    `json:"scope"`
	Unit        string                    `json:"unit,omitempty"`
	Workflow    string                    `json:"workflow,omitempty"`
	Code        string                    `json:"code"`
	Message     string                    `json:"message"`
	Remediation string                    `json:"remediation"`
}

//nolint:govet // Field order preserves the stable JSON contract.
type integrationDoctorResult struct {
	Readiness          integrationDoctorReadiness      `json:"readiness"`
	Summary            integrationDoctorSummary        `json:"summary"`
	RemoteVerification integrationDoctorRemoteSummary  `json:"remote_verification"`
	Units              []integrationDoctorUnit         `json:"units"`
	Workflows          []integrationDoctorWorkflow     `json:"workflows"`
	Verifications      []integrationDoctorVerification `json:"verifications"`
	Diagnostics        []integrationDoctorDiagnostic   `json:"diagnostics"`
}

func newIntegrationDoctorDiagnostic(
	severity integrationDoctorSeverity,
	scope, code, message, remediation string,
) integrationDoctorDiagnostic {
	return integrationDoctorDiagnostic{
		Severity: severity, Scope: scope, Code: code, Message: message, Remediation: remediation,
	}
}

func finalizeIntegrationDoctorResult(result *integrationDoctorResult) {
	if result == nil {
		return
	}
	sort.SliceStable(result.Diagnostics, func(left, right int) bool {
		a := result.Diagnostics[left]
		b := result.Diagnostics[right]
		if severityRank(a.Severity) != severityRank(b.Severity) {
			return severityRank(a.Severity) < severityRank(b.Severity)
		}
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", a.Scope, a.Unit, a.Workflow, a.Code, a.Message) <
			fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", b.Scope, b.Unit, b.Workflow, b.Code, b.Message)
	})
	sort.SliceStable(result.Verifications, func(left, right int) bool {
		a := result.Verifications[left]
		b := result.Verifications[right]
		return fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", a.Workflow, a.Unit, a.Category, a.Subject, a.Evidence) <
			fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s", b.Workflow, b.Unit, b.Category, b.Subject, b.Evidence)
	})
	result.Summary = integrationDoctorSummary{}
	for _, verification := range result.Verifications {
		if verification.State == integrationDoctorVerified {
			result.Summary.Verified++
		}
	}
	for _, diagnostic := range result.Diagnostics {
		switch diagnostic.Severity {
		case integrationDoctorError:
			result.Summary.Errors++
		case integrationDoctorWarning:
			result.Summary.Warnings++
		case integrationDoctorRecommendation:
			result.Summary.Recommendations++
		case integrationDoctorNotVerifiable:
			result.Summary.NotVerifiable++
		}
	}
	switch {
	case result.Summary.Errors > 0:
		result.Readiness = integrationDoctorNotReady
	case result.Summary.Warnings > 0:
		result.Readiness = integrationDoctorReadyWithWarnings
	default:
		result.Readiness = integrationDoctorReady
	}
}

func severityRank(severity integrationDoctorSeverity) int {
	switch severity {
	case integrationDoctorError:
		return 0
	case integrationDoctorWarning:
		return 1
	case integrationDoctorRecommendation:
		return 2
	case integrationDoctorNotVerifiable:
		return 3
	default:
		return 4
	}
}
