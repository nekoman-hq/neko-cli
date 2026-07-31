package plugin

import "fmt"

// MaxExplicitExitCode is the largest portable process exit code accepted from
// a decoded plugin response.
const MaxExplicitExitCode = 125

// ValidateResponse checks transport invariants that must hold before any
// renderer writes response content. Status remains domain data except that an
// error response must carry the envelope required by every human renderer.
func ValidateResponse(response *Response) error {
	if response == nil {
		return fmt.Errorf("plugin response is nil")
	}
	if response.Status == "error" && response.Error == nil {
		return fmt.Errorf("plugin error response is missing its error envelope")
	}
	if code, present := response.ExplicitExitCode(); present && (code < 0 || code > MaxExplicitExitCode) {
		return fmt.Errorf("plugin response exit code %d is outside the supported range 0 through %d", code, MaxExplicitExitCode)
	}
	return nil
}
