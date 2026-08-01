package update

import "fmt"

type errorKind string

const (
	errorUnsupportedPlatform errorKind = "unsupported-platform"
	errorReleaseLookup       errorKind = "release-lookup"
	errorAssetMissing        errorKind = "asset-missing"
	errorChecksumMissing     errorKind = "checksum-missing"
	errorChecksumInvalid     errorKind = "checksum-invalid"
	errorChecksumMismatch    errorKind = "checksum-mismatch"
	errorArchiveInvalid      errorKind = "archive-invalid"
	errorManagerOwned        errorKind = "manager-owned"
	errorTargetUnreadable    errorKind = "target-unreadable"
	errorTargetUnsupported   errorKind = "target-unsupported"
	errorParentNotWritable   errorKind = "parent-not-writable"
	errorReservation         errorKind = "reservation"
	errorDownload            errorKind = "download"
	errorSiblingWrite        errorKind = "sibling-write"
	errorMode                errorKind = "mode"
	errorFileSync            errorKind = "file-sync"
	errorRename              errorKind = "rename"
	errorCleanup             errorKind = "cleanup"
	errorCommittedSync       errorKind = "committed-directory-sync"
	errorDowngrade           errorKind = "downgrade-refused"
)

type updateError struct {
	kind       errorKind
	message    string
	cause      error
	downloaded bool
	changed    bool
	cleanup    string
}

func (err *updateError) Error() string {
	message := err.message
	if err.cause != nil {
		message = fmt.Sprintf("%s: %v", message, err.cause)
	}
	if err.cleanup != "" {
		message = fmt.Sprintf("%s; cleanup: %s", message, err.cleanup)
	}
	return message
}

func (err *updateError) Unwrap() error {
	return err.cause
}

func newUpdateError(kind errorKind, message string, cause error) *updateError {
	return &updateError{kind: kind, message: message, cause: cause}
}
