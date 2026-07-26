package evidence

import (
	"strings"

	"github.com/nekoman-hq/neko-cli/pkg/log"
)

type evidenceArchiveProgressKind string

const (
	evidenceArchiveValidatingRequest evidenceArchiveProgressKind = "validating-request"
	evidenceArchiveRequestValidated  evidenceArchiveProgressKind = "request-validated"
	evidenceArchiveResolvingFamily   evidenceArchiveProgressKind = "resolving-family"
	evidenceArchiveReadingEvidence   evidenceArchiveProgressKind = "reading-evidence"
	evidenceArchiveResolvingIdentity evidenceArchiveProgressKind = "resolving-identity"
	evidenceArchiveVerifyingDigest   evidenceArchiveProgressKind = "verifying-digest"
	evidenceArchiveDigestVerified    evidenceArchiveProgressKind = "digest-verified"
	evidenceArchiveCheckingTarget    evidenceArchiveProgressKind = "checking-target"
	evidenceArchiveTargetAvailable   evidenceArchiveProgressKind = "target-available"
	evidenceArchivePreparingWrite    evidenceArchiveProgressKind = "preparing-write"
	evidenceArchiveWriteCompleted    evidenceArchiveProgressKind = "write-completed"
	evidenceArchiveVerifyingResult   evidenceArchiveProgressKind = "verifying-result"
	evidenceArchiveResultVerified    evidenceArchiveProgressKind = "result-verified"
	evidenceArchiveRemovingSource    evidenceArchiveProgressKind = "removing-source"
	evidenceArchiveSourceRemoved     evidenceArchiveProgressKind = "source-removed"
	evidenceArchiveCompleted         evidenceArchiveProgressKind = "completed"
	evidenceArchiveRefused           evidenceArchiveProgressKind = "refused"
)

type evidenceArchiveProgressEvent struct {
	Kind     evidenceArchiveProgressKind
	Family   string
	Identity string
	Digest   string
	Source   string
	Archive  string
	Phase    string
}

type evidenceArchiveProgress interface {
	ReportEvidenceArchiveProgress(evidenceArchiveProgressEvent)
}

type terminalEvidenceArchiveProgress struct{}

func (terminalEvidenceArchiveProgress) ReportEvidenceArchiveProgress(event evidenceArchiveProgressEvent) {
	message, values := renderEvidenceArchiveProgress(event)
	if message == "" {
		return
	}
	log.PluginV(log.Exec, message, values...)
}

func reportEvidenceArchiveProgress(progress evidenceArchiveProgress, event evidenceArchiveProgressEvent) {
	if progress != nil {
		progress.ReportEvidenceArchiveProgress(event)
	}
}

func renderEvidenceArchiveProgress(event evidenceArchiveProgressEvent) (string, []any) {
	switch event.Kind {
	case evidenceArchiveValidatingRequest:
		return "Validating evidence archive request", nil
	case evidenceArchiveRequestValidated:
		return "Archive request validated: family=%s identity=%s confirmation=explicit", []any{
			event.Family, abbreviatedEvidenceHash(event.Identity),
		}
	case evidenceArchiveResolvingFamily:
		return "Resolving archive evidence family: %s", []any{event.Family}
	case evidenceArchiveReadingEvidence:
		return "Reading and classifying local evidence for the selected family", nil
	case evidenceArchiveResolvingIdentity:
		return "Resolving exact archive identity: %s", []any{abbreviatedEvidenceHash(event.Identity)}
	case evidenceArchiveVerifyingDigest:
		return "Verifying current evidence digest: %s", []any{abbreviatedEvidenceHash(event.Digest)}
	case evidenceArchiveDigestVerified:
		return "Evidence digest verification completed", nil
	case evidenceArchiveCheckingTarget:
		return "Checking archive target: %s", []any{safeEvidenceArchiveHumanPath(event.Archive)}
	case evidenceArchiveTargetAvailable:
		return "Archive target is available", nil
	case evidenceArchivePreparingWrite:
		return "Preparing exact private archive write", nil
	case evidenceArchiveWriteCompleted:
		return "Exact private archive write completed: %s", []any{safeEvidenceArchiveHumanPath(event.Archive)}
	case evidenceArchiveVerifyingResult:
		return "Verifying archived evidence bytes", nil
	case evidenceArchiveResultVerified:
		return "Archived evidence bytes verified", nil
	case evidenceArchiveRemovingSource:
		return "Removing selected completed evidence source", nil
	case evidenceArchiveSourceRemoved:
		return "Selected completed evidence source removed", nil
	case evidenceArchiveCompleted:
		return "Evidence archive operation completed", nil
	case evidenceArchiveRefused:
		return "Evidence archive operation refused during %s", []any{archiveProgressPhase(event.Phase)}
	default:
		return "", nil
	}
}

func archiveProgressPhase(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "validation"
	}
	return value
}
