package githubdispatch

import (
	"fmt"
	"strings"
)

const maxStoredErrorLength = 1024

// BearerToken is a validated secret-bearing value whose formatting is always
// redacted.
type BearerToken struct {
	secret string
}

// NewBearerToken validates a token at the adapter boundary.
func NewBearerToken(value string) (BearerToken, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return BearerToken{}, fmt.Errorf("github actions dispatch token is missing")
	}
	return BearerToken{secret: value}, nil
}

func (token BearerToken) value() string {
	return token.secret
}

func (BearerToken) String() string {
	return "[redacted]"
}

func (BearerToken) GoString() string {
	return "[redacted]"
}

// SanitizeText bounds third-party text and removes the exact bearer secret.
func SanitizeText(value string, token BearerToken) string {
	value = CapText(value)
	if secret := strings.TrimSpace(token.value()); secret != "" {
		value = strings.ReplaceAll(value, secret, "[redacted]")
	}
	value = strings.ReplaceAll(value, "Bearer [redacted]", "Bearer [redacted]")
	return value
}

// CapText bounds a stored dispatch error using the existing byte contract.
func CapText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxStoredErrorLength {
		return value
	}
	return value[:maxStoredErrorLength] + "...[truncated]"
}
