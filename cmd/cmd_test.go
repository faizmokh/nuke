package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	output, err := executeCommand(rootCmd, "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, Version) {
		t.Errorf("version output = %q, want to contain %q", output, Version)
	}
}

func TestDerivedCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"derived"})
	if err != nil {
		t.Error("derived subcommand not registered")
	}
}

func TestSPMCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"spm"})
	if err != nil {
		t.Error("spm subcommand not registered")
	}
}

func TestAllCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"all"})
	if err != nil {
		t.Error("all subcommand not registered")
	}
}

func TestDerivedHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "derived", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "DerivedData") {
		t.Errorf("help output = %q, want to contain 'DerivedData'", output)
	}
}

func TestSPMHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "spm", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Package") {
		t.Errorf("help output = %q, want to contain 'SPM'", output)
	}
}

func TestAllHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "all", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "DerivedData") || !strings.Contains(output, "SPM") {
		t.Errorf("help output = %q, want to contain both targets", output)
	}
}
