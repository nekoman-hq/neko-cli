package githubdispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
)

const maxResponseBytes = 4096

// Outcome is the transport-level result of one workflow-dispatch request. It
// deliberately has no prepared or request-started lifecycle values.
type Outcome string

const (
	OutcomeAccepted Outcome = "accepted"
	OutcomeRejected Outcome = "rejected"
	OutcomeUnknown  Outcome = "unknown"
)

// Response contains bounded, sanitized HTTP facts only.
//
//nolint:govet // Fields follow result reporting order.
type Response struct {
	Outcome           Outcome
	HTTPStatus        int
	WorkflowRunID     string
	RunURL            string
	HTMLURL           string
	ResponseTimestamp string
	Error             string
}

func classifyResponse(response *http.Response, token BearerToken) Response {
	status := response.StatusCode
	body := readBoundedBody(response.Body)
	result := Response{
		HTTPStatus:        status,
		ResponseTimestamp: response.Header.Get("Date"),
	}
	switch {
	case status >= 200 && status <= 299:
		result.Outcome = OutcomeAccepted
		metadata := parseOptionalWorkflowRunMetadata(body)
		result.WorkflowRunID = metadata.runID
		result.RunURL = metadata.runURL
		result.HTMLURL = metadata.htmlURL
		if metadata.responseTimestamp != "" {
			result.ResponseTimestamp = metadata.responseTimestamp
		}
	case isDefinitiveRejectionStatus(status):
		result.Outcome = OutcomeRejected
		result.Error = SanitizeText(parseGitHubError(body, status), token)
	default:
		result.Outcome = OutcomeUnknown
		result.Error = SanitizeText(
			fmt.Sprintf("GitHub Actions dispatch returned HTTP %d; outcome is uncertain", status),
			token,
		)
	}
	return result
}

func isDefinitiveRejectionStatus(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound,
		http.StatusUnprocessableEntity, http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

type workflowRunMetadata struct {
	runID             string
	runURL            string
	htmlURL           string
	responseTimestamp string
}

func parseOptionalWorkflowRunMetadata(body []byte) workflowRunMetadata {
	if len(bytes.TrimSpace(body)) == 0 {
		return workflowRunMetadata{}
	}
	var payload struct {
		ID        json.Number `json:"id"`
		RunID     json.Number `json:"run_id"`
		RunURL    string      `json:"url"`
		HTMLURL   string      `json:"html_url"`
		CreatedAt string      `json:"created_at"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return workflowRunMetadata{}
	}
	runID := payload.RunID.String()
	if runID == "" {
		runID = payload.ID.String()
	}
	return workflowRunMetadata{
		runID:             runID,
		runURL:            payload.RunURL,
		htmlURL:           payload.HTMLURL,
		responseTimestamp: payload.CreatedAt,
	}
}

func parseGitHubError(body []byte, status int) string {
	var payload struct {
		Message          string `json:"message"`
		DocumentationURL string `json:"documentation_url"`
	}
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, &payload); err == nil && payload.Message != "" {
			message := payload.Message
			if payload.DocumentationURL != "" {
				message += " (" + payload.DocumentationURL + ")"
			}
			if status == http.StatusTooManyRequests {
				message += "; request was not accepted and requires a later explicit retry or resume decision"
			}
			return CapText(message)
		}
	}
	if status == http.StatusTooManyRequests {
		return "GitHub Actions dispatch was rejected with HTTP 429; request was not accepted and requires a later explicit retry or resume decision"
	}
	return fmt.Sprintf("GitHub Actions dispatch was rejected with HTTP %d", status)
}

func classifyTransportError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "GitHub Actions dispatch context was canceled after request start; outcome is uncertain"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "GitHub Actions dispatch timed out after request start; outcome is uncertain"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "GitHub Actions dispatch timed out after request start; outcome is uncertain"
	}
	return "GitHub Actions dispatch transport failed after request start; outcome is uncertain: " + err.Error()
}

func readBoundedBody(body io.Reader) []byte {
	if body == nil {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(body, maxResponseBytes))
	return data
}
