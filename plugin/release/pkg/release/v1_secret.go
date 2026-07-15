package release

import "strings"

// RedactV1ProcessResult removes known secrets from legacy executor output and
// errors while preserving the underlying process error for errors.Is/As.
func RedactV1ProcessResult(output []byte, processErr error, secrets ...string) ([]byte, error) {
	redacted := string(output)
	known := make([]string, 0, len(secrets))
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		known = append(known, secret)
		redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
	}
	if processErr == nil {
		return []byte(redacted), nil
	}
	return []byte(redacted), v1RedactedProcessError{cause: processErr, secrets: known}
}

// RedactV1ProcessResultFromEnvironment applies the legacy GITHUB_TOKEN
// contract without exposing a generic environment service.
func RedactV1ProcessResultFromEnvironment(output []byte, processErr error, environment []string) ([]byte, error) {
	for _, entry := range environment {
		if strings.HasPrefix(entry, "GITHUB_TOKEN=") {
			return RedactV1ProcessResult(output, processErr, strings.TrimPrefix(entry, "GITHUB_TOKEN="))
		}
	}
	return RedactV1ProcessResult(output, processErr)
}

type v1RedactedProcessError struct {
	cause   error
	secrets []string
}

func (err v1RedactedProcessError) Error() string {
	message := err.cause.Error()
	for _, secret := range err.secrets {
		message = strings.ReplaceAll(message, secret, "[REDACTED]")
	}
	return message
}

func (err v1RedactedProcessError) Unwrap() error { return err.cause }
