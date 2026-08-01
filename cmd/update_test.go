package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nekoman-hq/neko-cli/pkg/update"
	"github.com/spf13/cobra"
)

func TestCoreUpdateFailureRendersOnceAndUsesOrdinaryExit(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	previousExecute := executeCoreUpdate
	executeCoreUpdate = func(context.Context, update.CoreOptions) (update.CoreResult, error) {
		return update.CoreResult{}, errors.New("frozen update failure")
	}
	t.Cleanup(func() { executeCoreUpdate = previousExecute })
	updateForce = false
	updateDryRun = false

	root := &cobra.Command{Use: "neko"}
	command := newCoreUpdateCommand()
	root.AddCommand(command)
	var stderr bytes.Buffer
	root.SetErr(&stderr)
	root.SetOut(io.Discard)
	root.SetArgs([]string{"update"})
	err := root.Execute()
	if err == nil || ProcessExitCode(err) != 1 {
		t.Fatalf("error=%v exit=%d", err, ProcessExitCode(err))
	}
	if got := strings.Count(stderr.String(), "frozen update failure"); got != 1 {
		t.Fatalf("error occurrences=%d output=%q", got, stderr.String())
	}
	if strings.Contains(stderr.String(), "Usage:") || strings.Contains(stderr.String(), "goroutine") {
		t.Fatalf("failure output contains usage or stack: %q", stderr.String())
	}
}

func TestCoreUpdateForceTransportAndRedirectedOutput(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	previousExecute := executeCoreUpdate
	var received update.CoreOptions
	executeCoreUpdate = func(_ context.Context, opts update.CoreOptions) (update.CoreResult, error) {
		received = opts
		return update.CoreResult{
			Action:           update.ActionForcedReinstall,
			InstalledVersion: "1.2.3",
			SelectedVersion:  "1.2.3",
		}, nil
	}
	t.Cleanup(func() { executeCoreUpdate = previousExecute })
	updateForce = false
	updateDryRun = false

	root := &cobra.Command{Use: "neko"}
	root.AddCommand(newCoreUpdateCommand())
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"update", "--force"})
	output := captureUpdateStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("Execute: %v", err)
		}
	})
	if !received.Force || received.DryRun {
		t.Fatalf("received options=%#v", received)
	}
	if !strings.Contains(output, "Successfully reinstalled version 1.2.3") {
		t.Fatalf("output=%q", output)
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("redirected NO_COLOR output contains ANSI: %q", output)
	}
}

func captureUpdateStdout(t *testing.T, run func()) string {
	t.Helper()
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writeEnd
	defer func() { os.Stdout = original }()
	run()
	if closeErr := writeEnd.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	output, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatal(err)
	}
	if err := readEnd.Close(); err != nil {
		t.Fatal(err)
	}
	return string(output)
}
