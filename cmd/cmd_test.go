package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	return executeCommandWithInput(root, "", args...)
}

func executeCommandWithInput(root *cobra.Command, input string, args ...string) (string, error) {
	resetCommandFlags()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func resetCommandFlags() {
	yesFlag = false
	dryRunFlag = false
	derivedAllFlag = false
	derivedProjectFlag = ""
	derivedOlderThanFlag = ""
	derivedListFlag = false
	derivedInteractiveFlag = false
	resetHelpFlag(rootCmd)
	resetHelpFlag(derivedCmd)
	resetHelpFlag(spmCmd)
	resetHelpFlag(allCmd)
}

func resetHelpFlag(cmd *cobra.Command) {
	flag := cmd.Flags().Lookup("help")
	if flag == nil {
		return
	}
	_ = flag.Value.Set("false")
	flag.Changed = false
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
	for _, flag := range []string{"--all", "--project", "--older-than", "--list", "--interactive"} {
		if !strings.Contains(output, flag) {
			t.Errorf("help output = %q, want to contain %q", output, flag)
		}
	}
}

func TestDerivedCommandInteractiveSelection(t *testing.T) {
	originalTarget := DerivedTarget
	defer func() {
		DerivedTarget = originalTarget
	}()

	dir := t.TempDir()
	DerivedTarget.Path = dir
	if err := os.MkdirAll(filepath.Join(dir, "MyApp-abc123"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "OtherApp-def456"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MyApp-abc123", "main.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "OtherApp-def456", "main.txt"), []byte("world"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := executeCommandWithInput(rootCmd, "1\ny\n", "derived")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "MyApp-abc123")); !os.IsNotExist(err) {
		t.Fatalf("selected project still exists, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "OtherApp-def456")); err != nil {
		t.Fatalf("unselected project should remain: %v", err)
	}
}

func TestDerivedCommandListFlag(t *testing.T) {
	originalTarget := DerivedTarget
	defer func() {
		DerivedTarget = originalTarget
	}()

	dir := t.TempDir()
	DerivedTarget.Path = dir
	if err := os.MkdirAll(filepath.Join(dir, "MyApp-abc123"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MyApp-abc123", "main.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	output, err := executeCommand(rootCmd, "derived", "--list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "MyApp-abc123") {
		t.Fatalf("list output = %q, want project name", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "MyApp-abc123")); err != nil {
		t.Fatalf("list mode should not delete project: %v", err)
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
