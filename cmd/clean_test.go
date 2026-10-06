package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

func TestBareCleanRequiresTargetWithoutInput(t *testing.T) {
	original := isInteractiveTerminal
	t.Cleanup(func() { isInteractiveTerminal = original })
	for _, terminal := range []bool{false, true} {
		isInteractiveTerminal = func(io.Reader, io.Writer) bool { return terminal }
		for _, args := range [][]string{{"clean"}, {"clean", "--yes"}, {"clean", "--dry-run"}} {
			if terminal && len(args) == 1 {
				continue
			}
			root := NewRootCommand()
			root.SetIn(forbiddenInput{})
			root.SetOut(&bytes.Buffer{})
			root.SetArgs(args)
			if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "explicit cleanup target") {
				t.Fatalf("%v: %v", args, err)
			}
		}
	}
}

func TestTargetMenuDispatchAndCancel(t *testing.T) {
	originalTerminal, originalMenu := isInteractiveTerminal, runTargetMenu
	t.Cleanup(func() { isInteractiveTerminal = originalTerminal; runTargetMenu = originalMenu })
	isInteractiveTerminal = func(io.Reader, io.Writer) bool { return true }
	sentinel := errors.New("menu failed")
	for _, name := range []string{"derived", "caches", "spm", "archives", "device-support", "module-cache", "simulators", ""} {
		root := NewRootCommand()
		target, _, err := root.Find([]string{"clean", name})
		if err != nil {
			t.Fatal(err)
		}
		called := false
		if name != "" {
			target.RunE = func(c *cobra.Command, args []string) error {
				called = true
				if c.Context() == nil {
					t.Fatal("context lost")
				}
				if derivedTargetFor(c).Path != "/tmp/custom-derived" {
					t.Fatal("override lost")
				}
				if c.InOrStdin() == nil {
					t.Fatal("input lost")
				}
				return nil
			}
		}
		runTargetMenu = func(ctx context.Context, out io.Writer, in io.Reader, choices []tui.TargetChoice) (string, error) {
			if len(choices) != 7 {
				t.Fatal(choices)
			}
			return name, nil
		}
		if _, err := executeCommand(root, "clean", "--derived-data", "/tmp/custom-derived"); err != nil {
			t.Fatal(err)
		}
		if called != (name != "") {
			t.Fatalf("dispatch %q: %v", name, called)
		}
	}
	runTargetMenu = func(context.Context, io.Writer, io.Reader, []tui.TargetChoice) (string, error) { return "", sentinel }
	if _, err := executeCommand(NewRootCommand(), "clean"); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}

func TestRemovedInterfaceAndFlagPlacement(t *testing.T) {
	for _, args := range [][]string{{"derived"}, {"all"}, {"spm"}, {"archives"}, {"device-support"}, {"module-cache"}, {"simulators"}, {"clean", "derived", "--interactive"}, {"status", "--yes"}, {"doctor", "--dry-run"}} {
		if _, err := executeCommand(NewRootCommand(), args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{}, {"--help"}} {
		out, err := executeCommand(NewRootCommand(), args...)
		if err != nil || !strings.Contains(out, "nuke clean") || !strings.Contains(out, "doctor") {
			t.Fatalf("%q: %v", out, err)
		}
	}
}

func TestMenuUsesModuleCacheOverrideAndConfirmation(t *testing.T) {
	originalTerminal, originalMenu := isInteractiveTerminal, runTargetMenu
	t.Cleanup(func() { isInteractiveTerminal = originalTerminal; runTargetMenu = originalMenu })
	isInteractiveTerminal = func(io.Reader, io.Writer) bool { return true }
	runTargetMenu = func(context.Context, io.Writer, io.Reader, []tui.TargetChoice) (string, error) {
		return "module-cache", nil
	}
	rootPath := t.TempDir()
	file := filepath.Join(rootPath, "ModuleCache.noindex", "module.pcm")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("cache"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := executeCommandWithInput(NewRootCommand(), "n\n", "clean", "--derived-data", rootPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "5 B") || !strings.Contains(out, "[y/N]") {
		t.Fatal(out)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("declined cleanup removed file", err)
	}
}

func TestCompletionUsesCleanHierarchy(t *testing.T) {
	out, err := executeCommand(NewRootCommand(), "__complete", "clean", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"derived", "caches", "spm", "archives", "device-support", "module-cache", "simulators"} {
		if !strings.Contains(out, name+"\t") {
			t.Fatalf("missing %s: %s", name, out)
		}
	}
	out, err = executeCommand(NewRootCommand(), "__complete", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "derived\t") || strings.Contains(out, "all\t") {
		t.Fatal(out)
	}
}
