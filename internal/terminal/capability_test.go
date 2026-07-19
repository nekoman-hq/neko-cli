package terminal

import (
	"bytes"
	"os"
	"testing"
)

func TestColorPolicyRequiresTTYAndHonorsNOColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		writer      anyWriter
		noColor     string
		noColorSet  bool
		terminal    bool
		wantEnabled bool
	}{
		{name: "interactive terminal", writer: fileDescriptorWriter(7), terminal: true, wantEnabled: true},
		{name: "redirected file", writer: fileDescriptorWriter(7), terminal: false},
		{name: "writer without descriptor", writer: &bytes.Buffer{}, terminal: true},
		{name: "NO_COLOR", writer: fileDescriptorWriter(7), noColor: "1", noColorSet: true, terminal: true},
		{name: "empty NO_COLOR", writer: fileDescriptorWriter(7), noColorSet: true, terminal: true, wantEnabled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := colorPolicy{
				lookupEnv: func(string) (string, bool) { return test.noColor, test.noColorSet },
				isTerminal: func(fd int) bool {
					if fd != 7 {
						t.Fatalf("terminal probe fd = %d, want 7", fd)
					}
					return test.terminal
				},
			}
			if got := policy.enabled(test.writer); got != test.wantEnabled {
				t.Fatalf("enabled = %t, want %t", got, test.wantEnabled)
			}
		})
	}
}

func TestColorPolicyReadsNOColorFromEnvironment(t *testing.T) {
	t.Setenv("NO_COLOR", "enabled")
	policy := colorPolicy{
		lookupEnv: os.LookupEnv,
		isTerminal: func(int) bool {
			return true
		},
	}
	if policy.enabled(fileDescriptorWriter(7)) {
		t.Fatal("NO_COLOR did not disable interactive color")
	}
}

type anyWriter interface {
	Write([]byte) (int, error)
}

type fileDescriptorWriter uintptr

func (writer fileDescriptorWriter) Write(value []byte) (int, error) {
	return len(value), nil
}

func (writer fileDescriptorWriter) Fd() uintptr {
	return uintptr(writer)
}
