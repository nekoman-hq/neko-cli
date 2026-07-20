package workflowinit

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type workflowInitClock interface {
	Now() time.Time
}

type systemWorkflowInitClock struct{}

func (systemWorkflowInitClock) Now() time.Time {
	return time.Now()
}

type commandFailure struct {
	Cause   error
	Details map[string]any
	Code    string
	Message string
}

func failureFromError(code string, cause error) *commandFailure {
	return &commandFailure{Code: code, Cause: cause}
}

func failureFromMessage(code, message string) *commandFailure {
	return &commandFailure{Code: code, Message: message}
}

func (failure *commandFailure) responseMessage() string {
	if failure == nil {
		return ""
	}
	if failure.Cause != nil {
		return failure.Cause.Error()
	}
	return failure.Message
}

func mapCommandFailure(command string, failure *commandFailure, timestamp time.Time) *plugin.Response {
	if failure == nil {
		return nil
	}
	return &plugin.Response{
		Status:   "error",
		Metadata: workflowInitResponseMetadata(command, timestamp),
		Error: &plugin.ResponseError{
			Code: failure.Code, Message: failure.responseMessage(),
			Details: cloneResponseDetails(failure.Details),
		},
	}
}

func workflowInitResponseMetadata(command string, timestamp time.Time) plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin: metadata.PluginName, Version: metadata.Version,
		Command: command, Timestamp: timestamp,
	}
}

func cloneResponseDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}
	clone := make(map[string]any, len(details))
	for key, value := range details {
		clone[key] = value
	}
	return clone
}
