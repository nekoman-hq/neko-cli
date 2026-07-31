package plugin_test

import (
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/plugin"
)

func TestValidateResponseRejectsProtocolViolationsWithoutDerivingExitFromStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response *plugin.Response
		want     string
	}{
		{name: "nil response", want: "response is nil"},
		{name: "missing error envelope", response: &plugin.Response{Status: "error"}, want: "missing its error envelope"},
		{name: "negative exit", response: explicitValidationResponse(-1), want: "supported range 0 through 125"},
		{name: "exit above range", response: explicitValidationResponse(126), want: "supported range 0 through 125"},
		{name: "valid explicit zero", response: explicitValidationResponse(0)},
		{name: "valid explicit maximum", response: explicitValidationResponse(125)},
		{name: "legacy error remains compatible", response: &plugin.Response{Status: "error", Error: &plugin.ResponseError{Code: "LEGACY", Message: "legacy"}}},
		{name: "status has no exit meaning", response: &plugin.Response{Status: "warning"}},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := plugin.ValidateResponse(test.response)
			if test.want == "" && err != nil {
				t.Fatalf("ValidateResponse: %v", err)
			}
			if test.want != "" && (err == nil || !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("ValidateResponse error = %v, want fragment %q", err, test.want)
			}
		})
	}
}

func explicitValidationResponse(code int) *plugin.Response {
	response := &plugin.Response{Status: "success"}
	response.SetExitCode(code)
	return response
}
