package contextvalidation

import (
	"time"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
	"github.com/nekoman-hq/neko-cli/plugin/release/pkg/metadata"
)

type contextValidationClock interface {
	Now() time.Time
}

type systemContextValidationClock struct{}

func (systemContextValidationClock) Now() time.Time {
	return time.Now()
}

type commandFailure struct {
	Code    string
	Message string
	Context *ValidatedReleaseContext
}

func failureFromMessage(code, message string) *commandFailure {
	return &commandFailure{Code: code, Message: message}
}

func (failure *commandFailure) responseMessage() string {
	if failure == nil {
		return ""
	}
	return failure.Message
}

func mapCommandFailure(command string, failure *commandFailure, timestamp time.Time) *plugin.Response {
	if failure == nil {
		return nil
	}
	return &plugin.Response{
		Status:   "error",
		Metadata: contextValidationResponseMetadata(command, timestamp),
		Error: &plugin.ResponseError{
			Code: failure.Code, Message: failure.Message,
		},
	}
}

func contextValidationResponseMetadata(command string, timestamp time.Time) plugin.ResponseMetadata {
	return plugin.ResponseMetadata{
		Plugin: metadata.PluginName, Version: metadata.Version,
		Command: command, Timestamp: timestamp,
	}
}

func contextValidationFlagString(flags map[string]any, name string) string {
	value, _ := flags[name].(string)
	return value
}
