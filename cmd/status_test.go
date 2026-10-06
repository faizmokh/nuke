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

	"github.com/faizmokh/nuke/internal"
)

func setupStatus(t *testing.T) string {
	t.Helper()
	derived, spm, archives, device, module := DerivedTarget, SPMTarget, ArchivesTarget, DeviceSupportTarget, ModuleCacheTarget
	list := listUnavailableSimulators
	t.Cleanup(func() {
		DerivedTarget = derived
		SPMTarget = spm
		ArchivesTarget = archives
		DeviceSupportTarget = device
		ModuleCacheTarget = module
		listUnavailableSimulators = list
	})
	dir := t.TempDir()
	DerivedTarget.Path = filepath.Join(dir, "DerivedData")
	SPMTarget.Path = filepath.Join(dir, "clean", "spm")
	ArchivesTarget.Path = filepath.Join(dir, "clean", "archives")
	DeviceSupportTarget.Path = filepath.Join(dir, "support")
	ModuleCacheTarget.Path = filepath.Join(DerivedTarget.Path, "ModuleCache.noindex")
	listUnavailableSimulators = func(context.Context) ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "Unavailable", UDID: "ABC"}}, nil
	}
	return dir
}

func statusFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusReportsSizesWithoutDoubleCountingOrReadingInput(t *testing.T) {
	setupStatus(t)
	statusFile(t, filepath.Join(DerivedTarget.Path, "App", "file"), "123")
	statusFile(t, filepath.Join(ModuleCacheTarget.Path, "UIKit.pcm"), "12345")
	statusFile(t, filepath.Join(SPMTarget.Path, "cache"), "12")
	statusFile(t, filepath.Join(ArchivesTarget.Path, "archive"), "1234")
	for _, args := range [][]string{{"status"}} {
		root := NewRootCommand()
		var out bytes.Buffer
		root.SetIn(forbiddenInput{})
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		text := out.String()
		for _, want := range []string{"DerivedData", "8 B", "5 B", "included in DerivedData", "not present", "Unavailable simulators", "Total estimated filesystem size: 14 B"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in %s", want, text)
			}
		}
		if strings.ContainsAny(text, "\r\x1b") || strings.Contains(text, "[y/N]") {
			t.Fatalf("unexpected interactive output: %q", text)
		}
	}
	for path, want := range map[string]string{filepath.Join(DerivedTarget.Path, "App", "file"): "123", filepath.Join(ModuleCacheTarget.Path, "UIKit.pcm"): "12345", filepath.Join(SPMTarget.Path, "cache"): "12", filepath.Join(ArchivesTarget.Path, "archive"): "1234"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("file changed: %s: %q %v", path, data, err)
		}
	}
}

func TestStatusMissingTargetsAreNormal(t *testing.T) {
	setupStatus(t)
	text, err := executeCommand(NewRootCommand(), "status")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(text, "not present") != 5 || !strings.Contains(text, "Total estimated filesystem size: 0 B") {
		t.Fatal(text)
	}
}

func TestStatusContinuesAfterScanFailure(t *testing.T) {
	setupStatus(t)
	// A file cannot be scanned as a target directory; deterministic on all permissions.
	statusFile(t, DerivedTarget.Path, "invalid target")
	statusFile(t, filepath.Join(SPMTarget.Path, "cache"), "123")
	text, err := executeCommand(NewRootCommand(), "status")
	if err == nil {
		t.Fatal("scan failure ignored")
	}
	for _, want := range []string{"partial / unavailable", "SPM caches", "3 B", "Known estimated filesystem size (partial): 3 B", "Unavailable simulators"} {
		if !strings.Contains(text, want) {
			t.Fatal(text)
		}
	}
}

func TestStatusSimulatorFailureRetainsFilesystemSummary(t *testing.T) {
	setupStatus(t)
	failure := errors.New("Xcode unavailable")
	listUnavailableSimulators = func(context.Context) ([]internal.SimulatorDevice, error) { return nil, failure }
	text, err := executeCommand(NewRootCommand(), "status")
	if !errors.Is(err, failure) || !strings.Contains(text, "Total estimated filesystem size: 0 B") {
		t.Fatalf("text=%s err=%v", text, err)
	}
}

func TestStatusCancellationSkipsSimulatorCommand(t *testing.T) {
	setupStatus(t)
	listUnavailableSimulators = func(context.Context) ([]internal.SimulatorDevice, error) {
		t.Fatal("simulator command called after cancellation")
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := NewRootCommand()
	root.SetIn(forbiddenInput{})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"status"})
	if err := root.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestStatusStyledOutputAndHelp(t *testing.T) {
	setupStatus(t)
	interactive := isInteractiveTerminal
	t.Cleanup(func() { isInteractiveTerminal = interactive })
	isInteractiveTerminal = func(io.Reader, io.Writer) bool { return true }
	text, err := executeCommand(NewRootCommand(), "status")
	if err != nil || !strings.Contains(text, "Cache Status") || !strings.Contains(text, "╭") {
		t.Fatalf("text=%s err=%v", text, err)
	}
	text, err = executeCommand(NewRootCommand(), "status", "--help")
	if err != nil || !strings.Contains(text, "Nothing is deleted") {
		t.Fatalf("text=%s err=%v", text, err)
	}
	if _, err = executeCommand(NewRootCommand(), "status", "unexpected"); err == nil {
		t.Fatal("unexpected args accepted")
	}
}

func TestStatusSeparateModuleCacheContributesToTotal(t *testing.T) {
	dir := setupStatus(t)
	ModuleCacheTarget.Path = filepath.Join(dir, "separate-module-cache")
	statusFile(t, filepath.Join(DerivedTarget.Path, "App", "file"), "123")
	statusFile(t, filepath.Join(ModuleCacheTarget.Path, "UIKit.pcm"), "12345")
	text, err := executeCommand(NewRootCommand(), "status")
	if err != nil || !strings.Contains(text, "Total estimated filesystem size: 8 B") || strings.Contains(text, "included in DerivedData") {
		t.Fatalf("text=%s err=%v", text, err)
	}
}
