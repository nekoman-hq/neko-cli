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
	var err error
	stdout := captureUpdateStdout(t, func() { err = root.Execute() })
	if err == nil || ProcessExitCode(err) != 1 {
		t.Fatalf("error=%v exit=%d", err, ProcessExitCode(err))
	}
	if strings.Count(stdout, "Checking for neko-cli updates...") != 1 {
		t.Fatalf("update progress output=%q", stdout)
	}
	if got := strings.Count(stderr.String(), "frozen update failure"); got != 1 {
		t.Fatalf("error occurrences=%d output=%q", got, stderr.String())
	}
	if strings.Contains(stderr.String(), "Usage:") || strings.Contains(stderr.String(), "goroutine") {
		t.Fatalf("failure output contains usage or stack: %q", stderr.String())
	}
}

func TestNonUpdateCommandDoesNotInvokeCoreUpdateLookup(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	previousExecute := executeCoreUpdate
	lookupCalls := 0
	executeCoreUpdate = func(context.Context, update.CoreOptions) (update.CoreResult, error) {
		lookupCalls++
		return update.CoreResult{}, errors.New("unexpected update lookup")
	}
	t.Cleanup(func() { executeCoreUpdate = previousExecute })

	root := &cobra.Command{Use: "neko"}
	root.AddCommand(newCoreUpdateCommand())
	root.AddCommand(&cobra.Command{Use: "inspect", RunE: func(*cobra.Command, []string) error { return nil }})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"inspect"})
	stdout := captureUpdateStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatalf("execute non-update command: %v", err)
		}
	})
	if lookupCalls != 0 {
		t.Fatalf("non-update command invoked update lookup %d time(s)", lookupCalls)
	}
	if strings.Contains(stdout, "Checking for neko-cli updates...") {
		t.Fatalf("non-update command emitted update progress: %q", stdout)
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
