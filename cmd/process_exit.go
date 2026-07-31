package cmd

import (
	"errors"
	"fmt"
)

type responseProcessExitError struct {
	code int
}

func (exitError *responseProcessExitError) Error() string {
	return fmt.Sprintf("plugin response requested process exit %d", exitError.code)
}

// ProcessExitCode resolves the final executable status for an error returned
// by Execute. Only validated response-owned exits retain an exact code;
// ordinary command, transport, and renderer failures use exit code 1.
func ProcessExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitError *responseProcessExitError
	if errors.As(err, &exitError) {
		return exitError.code
	}
	return 1
}

func newResponseProcessExitError(code int) error {
	return &responseProcessExitError{code: code}
}

func isResponseProcessExitError(err error) bool {
	var exitError *responseProcessExitError
	return errors.As(err, &exitError)
}
