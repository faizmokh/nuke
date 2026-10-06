package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizmokh/nuke/internal"
)

func TestDoctorReadOnlyAndCustomRoot(t *testing.T) {
	original := runDoctorChecks
	t.Cleanup(func() { runDoctorChecks = original })
	path := t.TempDir()
	runDoctorChecks = func(ctx context.Context, targets []internal.Target) ([]internal.Finding, error) {
		if targets[0].Path != path || targets[4].Path != filepath.Join(path, "ModuleCache.noindex") {
			t.Fatal(targets)
		}
		return []internal.Finding{{Name: "iOS SDKs", State: "OK", Detail: "available"}, {Name: "Disk space", State: "WARN", Detail: "low", Advice: "Review nuke status"}}, nil
	}
	for _, args := range [][]string{{"doctor", "--derived-data", path}, {"--derived-data", path, "doctor"}} {
		root := NewRootCommand()
		var out bytes.Buffer
		root.SetIn(forbiddenInput{})
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"[OK] iOS SDKs", "[WARN] Disk space", "Next: Review nuke status", "Doctor: 0 errors, 1 warnings"} {
			if !strings.Contains(out.String(), want) {
				t.Fatal(out.String())
			}
		}
		if strings.ContainsAny(out.String(), "\x1b\r") || strings.Contains(out.String(), "[y/N]") {
			t.Fatal(out.String())
		}
	}
}
func TestDoctorDisplaysErrorsAndReturnsFailure(t *testing.T) {
	original := runDoctorChecks
	t.Cleanup(func() { runDoctorChecks = original })
	failure := errors.New("Xcode unavailable")
	runDoctorChecks = func(context.Context, []internal.Target) ([]internal.Finding, error) {
		return []internal.Finding{{Name: "Xcode", State: "ERROR", Detail: "unavailable", Advice: "Select Xcode"}}, failure
	}
	text, err := executeCommand(NewRootCommand(), "doctor")
	if !errors.Is(err, failure) || !strings.Contains(text, "Doctor: 1 errors, 0 warnings") {
		t.Fatal(text, err)
	}
}
func TestDoctorStyledOutputAndHelp(t *testing.T) {
	original, interactive := runDoctorChecks, isInteractiveTerminal
	t.Cleanup(func() { runDoctorChecks = original; isInteractiveTerminal = interactive })
	runDoctorChecks = func(context.Context, []internal.Target) ([]internal.Finding, error) { return nil, nil }
	isInteractiveTerminal = func(io.Reader, io.Writer) bool { return true }
	text, err := executeCommand(NewRootCommand(), "doctor")
	if err != nil || !strings.Contains(text, "iOS Development Doctor") || !strings.Contains(text, "╭") {
		t.Fatal(text, err)
	}
	text, err = executeCommand(NewRootCommand(), "doctor", "--help")
	if err != nil || !strings.Contains(text, "Never installs tools") {
		t.Fatal(text, err)
	}
	if _, err = executeCommand(NewRootCommand(), "doctor", "extra"); err == nil {
		t.Fatal("unexpected argument accepted")
	}
}
