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
	"time"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	return executeCommandWithInput(root, "", args...)
}

func executeCommandWithInput(root *cobra.Command, input string, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetIn(strings.NewReader(input))
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, Version) {
		t.Errorf("version output = %q, want to contain %q", output, Version)
	}
}

func TestDerivedCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "derived"})
	if err != nil {
		t.Error("derived subcommand not registered")
	}
}

func TestSPMCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "spm"})
	if err != nil {
		t.Error("spm subcommand not registered")
	}
}

func TestArchivesCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "archives"})
	if err != nil {
		t.Error("archives subcommand not registered")
	}
}

func TestDeviceSupportCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "device-support"})
	if err != nil {
		t.Error("device-support subcommand not registered")
	}
}

func TestModuleCacheCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "module-cache"})
	if err != nil {
		t.Error("module-cache subcommand not registered")
	}
}

func TestSimulatorsCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "simulators"})
	if err != nil {
		t.Error("simulators subcommand not registered")
	}
}

func TestAllCommandRegistered(t *testing.T) {
	_, _, err := NewRootCommand().Find([]string{"clean", "caches"})
	if err != nil {
		t.Error("all subcommand not registered")
	}
}

func TestDerivedHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "derived", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "DerivedData") {
		t.Errorf("help output = %q, want to contain 'DerivedData'", output)
	}
	for _, flag := range []string{"--all", "--project", "--older-than", "--list", "--select"} {
		if !strings.Contains(output, flag) {
			t.Errorf("help output = %q, want to contain %q", output, flag)
		}
	}
}

func TestRootHelpListsExpandedCleanupTargets(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"clean", "status", "doctor"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output = %q, want to contain %q", output, want)
		}
	}
	if !strings.Contains(output, "Xcode and iOS development caches") {
		t.Fatalf("help output = %q, want expanded root description", output)
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

	_, err := executeCommandWithInput(NewRootCommand(), "1\ny\n", "clean", "derived")
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

	output, err := executeCommand(NewRootCommand(), "clean", "derived", "--list")
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

func TestDerivedCommandUsesPickerForInteractiveTerminal(t *testing.T) {
	originalTarget := DerivedTarget
	originalInteractive := isInteractiveTerminal
	originalPicker := runDerivedPicker
	defer func() {
		DerivedTarget = originalTarget
		isInteractiveTerminal = originalInteractive
		runDerivedPicker = originalPicker
	}()

	dir := t.TempDir()
	DerivedTarget.Path = dir
	if err := os.MkdirAll(filepath.Join(dir, "MyApp-abc123"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MyApp-abc123", "main.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	pickerCalled := false
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	runDerivedPicker = func(ctx context.Context, out io.Writer, in io.Reader, target internal.Target, options internal.ScanOptions) (internal.ScanPlan, error) {
		pickerCalled = true
		return internal.ScanPlan{}, nil
	}

	_, err := executeCommand(NewRootCommand(), "clean", "derived")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pickerCalled {
		t.Fatal("expected interactive derived command to use picker")
	}
}

func TestDerivedCommandListAndYesBypassPicker(t *testing.T) {
	originalTarget := DerivedTarget
	originalInteractive := isInteractiveTerminal
	originalPicker := runDerivedPicker
	defer func() {
		DerivedTarget = originalTarget
		isInteractiveTerminal = originalInteractive
		runDerivedPicker = originalPicker
	}()

	dir := t.TempDir()
	DerivedTarget.Path = dir
	if err := os.MkdirAll(filepath.Join(dir, "MyApp-abc123"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "MyApp-abc123", "main.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	pickerCalled := false
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	runDerivedPicker = func(ctx context.Context, out io.Writer, in io.Reader, target internal.Target, options internal.ScanOptions) (internal.ScanPlan, error) {
		pickerCalled = true
		return internal.ScanPlan{}, nil
	}

	if _, err := executeCommand(NewRootCommand(), "clean", "derived", "--list"); err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if pickerCalled {
		t.Fatal("expected --list to bypass picker")
	}

	pickerCalled = false
	if _, err := executeCommand(NewRootCommand(), "clean", "derived", "--yes"); err != nil {
		t.Fatalf("unexpected yes error: %v", err)
	}
	if pickerCalled {
		t.Fatal("expected --yes to bypass picker")
	}
}

func TestDerivedCommandInteractiveNoMatchesShowsNothingToClean(t *testing.T) {
	originalInteractive := isInteractiveTerminal
	originalPicker := runDerivedPicker
	defer func() {
		isInteractiveTerminal = originalInteractive
		runDerivedPicker = originalPicker
	}()

	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	runDerivedPicker = func(ctx context.Context, out io.Writer, in io.Reader, target internal.Target, options internal.ScanOptions) (internal.ScanPlan, error) {
		return internal.ScanPlan{}, tui.ErrNoEntries
	}

	output, err := executeCommand(NewRootCommand(), "clean", "derived", "--select")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "nothing to clean") {
		t.Fatalf("output = %q, want nothing to clean message", output)
	}
}

func TestSPMHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "spm", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Package") {
		t.Errorf("help output = %q, want to contain 'SPM'", output)
	}
}

func TestArchivesHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "archives", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Archives") {
		t.Errorf("help output = %q, want to contain 'Archives'", output)
	}
}

func TestDeviceSupportHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "device-support", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "DeviceSupport") {
		t.Errorf("help output = %q, want to contain 'DeviceSupport'", output)
	}
}

func TestModuleCacheHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "module-cache", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "module cache") {
		t.Errorf("help output = %q, want to contain 'module cache'", output)
	}
}

func TestSimulatorsHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "simulators", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "unavailable simulators") {
		t.Errorf("help output = %q, want to contain 'unavailable simulators'", output)
	}
}

func TestArchivesCommandSkipConfirm(t *testing.T) {
	originalTarget := ArchivesTarget
	originalInteractive := isInteractiveTerminal
	defer func() {
		ArchivesTarget = originalTarget
		isInteractiveTerminal = originalInteractive
	}()

	dir := t.TempDir()
	ArchivesTarget.Path = dir
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	if err := os.MkdirAll(filepath.Join(dir, "2026-05-16"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-05-16", "app.xcarchive"), []byte("archive"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := executeCommand(NewRootCommand(), "clean", "archives", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("archives target left %d entries, want 0", len(entries))
	}
}

func TestArchivesCommandInteractiveDryRunUsesStyledSummary(t *testing.T) {
	originalTarget := ArchivesTarget
	originalInteractive := isInteractiveTerminal
	defer func() {
		ArchivesTarget = originalTarget
		isInteractiveTerminal = originalInteractive
	}()

	dir := t.TempDir()
	ArchivesTarget.Path = dir
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	if err := os.MkdirAll(filepath.Join(dir, "2026-05-16"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "2026-05-16", "app.xcarchive"), []byte("archive"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	output, err := executeCommand(NewRootCommand(), "clean", "archives", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Archives") || !strings.Contains(output, "Reclaimable") || !strings.Contains(output, "╭") {
		t.Fatalf("output = %q, want styled archives summary card", output)
	}
	if _, err := os.Stat(filepath.Join(dir, "2026-05-16")); err != nil {
		t.Fatalf("dry run should not delete archives: %v", err)
	}
}

func TestDeviceSupportCommandSkipConfirm(t *testing.T) {
	originalTarget := DeviceSupportTarget
	defer func() {
		DeviceSupportTarget = originalTarget
	}()

	dir := t.TempDir()
	DeviceSupportTarget.Path = dir
	if err := os.MkdirAll(filepath.Join(dir, "17.5 (21F79)"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "17.5 (21F79)", "Symbols"), []byte("symbols"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := executeCommand(NewRootCommand(), "clean", "device-support", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("device-support target left %d entries, want 0", len(entries))
	}
}

func TestModuleCacheCommandSkipConfirm(t *testing.T) {
	originalTarget := ModuleCacheTarget
	defer func() {
		ModuleCacheTarget = originalTarget
	}()

	dir := t.TempDir()
	ModuleCacheTarget.Path = dir
	if err := os.MkdirAll(filepath.Join(dir, "ABC123"), 0755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ABC123", "UIKit.pcm"), []byte("pcm"), 0644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	_, err := executeCommand(NewRootCommand(), "clean", "module-cache", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("module-cache target left %d entries, want 0", len(entries))
	}
}

func TestSimulatorsCommandDryRun(t *testing.T) {
	originalList := listUnavailableSimulators
	originalDelete := deleteUnavailableSimulators
	defer func() {
		listUnavailableSimulators = originalList
		deleteUnavailableSimulators = originalDelete
	}()

	deleteCalled := false
	listUnavailableSimulators = func(ctx context.Context) ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{
			{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"},
			{Name: "iPad Pro", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-16-4", UDID: "DEF"},
		}, nil
	}
	deleteUnavailableSimulators = func(ctx context.Context) error {
		deleteCalled = true
		return nil
	}

	output, err := executeCommand(NewRootCommand(), "clean", "simulators", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"iPhone 8", "iPad Pro", "2 unavailable simulators"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want to contain %q", output, want)
		}
	}
	if deleteCalled {
		t.Fatal("dry run should not delete simulators")
	}
}

func TestSimulatorsCommandSkipConfirm(t *testing.T) {
	originalList := listUnavailableSimulators
	originalDelete := deleteUnavailableSimulators
	originalInteractive := isInteractiveTerminal
	defer func() {
		listUnavailableSimulators = originalList
		deleteUnavailableSimulators = originalDelete
		isInteractiveTerminal = originalInteractive
	}()

	deleteCalled := false
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	listUnavailableSimulators = func(ctx context.Context) ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"}}, nil
	}
	deleteUnavailableSimulators = func(ctx context.Context) error {
		deleteCalled = true
		return nil
	}

	output, err := executeCommand(NewRootCommand(), "clean", "simulators", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected deleteUnavailableSimulators to be called")
	}
	if !strings.Contains(output, "Deleted 1 unavailable simulator") {
		t.Fatalf("output = %q, want deletion confirmation", output)
	}
}

func TestSimulatorsCommandInteractiveDryRunUsesStyledSummary(t *testing.T) {
	originalList := listUnavailableSimulators
	originalDelete := deleteUnavailableSimulators
	originalInteractive := isInteractiveTerminal
	defer func() {
		listUnavailableSimulators = originalList
		deleteUnavailableSimulators = originalDelete
		isInteractiveTerminal = originalInteractive
	}()

	deleteCalled := false
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}
	listUnavailableSimulators = func(ctx context.Context) ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"}}, nil
	}
	deleteUnavailableSimulators = func(ctx context.Context) error {
		deleteCalled = true
		return nil
	}

	output, err := executeCommand(NewRootCommand(), "clean", "simulators", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Unavailable Simulators") || !strings.Contains(output, "iPhone 8") || !strings.Contains(output, "╭") {
		t.Fatalf("output = %q, want styled simulator summary card", output)
	}
	if deleteCalled {
		t.Fatal("dry run should not delete simulators")
	}
}

func TestSimulatorsCommandListError(t *testing.T) {
	originalList := listUnavailableSimulators
	defer func() {
		listUnavailableSimulators = originalList
	}()

	listUnavailableSimulators = func(ctx context.Context) ([]internal.SimulatorDevice, error) {
		return nil, errors.New("simctl exploded")
	}

	_, err := executeCommand(NewRootCommand(), "clean", "simulators")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "listing unavailable simulators") {
		t.Fatalf("error = %q, want list context", err)
	}
}

func TestSimulatorsCommandDeleteError(t *testing.T) {
	originalList := listUnavailableSimulators
	originalDelete := deleteUnavailableSimulators
	defer func() {
		listUnavailableSimulators = originalList
		deleteUnavailableSimulators = originalDelete
	}()

	listUnavailableSimulators = func(ctx context.Context) ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"}}, nil
	}
	deleteUnavailableSimulators = func(ctx context.Context) error {
		return errors.New("permission denied")
	}

	_, err := executeCommand(NewRootCommand(), "clean", "simulators", "--yes")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "deleting unavailable simulators") {
		t.Fatalf("error = %q, want delete context", err)
	}
}

func TestAllHelp(t *testing.T) {
	output, err := executeCommand(NewRootCommand(), "clean", "caches", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "DerivedData") || !strings.Contains(output, "SPM") {
		t.Errorf("help output = %q, want to contain both targets", output)
	}
}

func TestAllCommandInteractiveDryRunUsesStyledSummary(t *testing.T) {
	originalDerived := DerivedTarget
	originalSPM := SPMTarget
	originalInteractive := isInteractiveTerminal
	defer func() {
		DerivedTarget = originalDerived
		SPMTarget = originalSPM
		isInteractiveTerminal = originalInteractive
	}()

	dir := t.TempDir()
	derivedDir := filepath.Join(dir, "DerivedData")
	spmDir := filepath.Join(dir, "swiftpm")
	DerivedTarget.Path = derivedDir
	SPMTarget.Path = spmDir
	isInteractiveTerminal = func(in io.Reader, out io.Writer) bool {
		return true
	}

	if err := os.MkdirAll(filepath.Join(derivedDir, "MyApp-abc123"), 0755); err != nil {
		t.Fatalf("MkdirAll(derived) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(derivedDir, "MyApp-abc123", "main.txt"), []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile(derived) error: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(spmDir, "artifacts"), 0755); err != nil {
		t.Fatalf("MkdirAll(spm) error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(spmDir, "artifacts", "cache.db"), []byte("cache"), 0644); err != nil {
		t.Fatalf("WriteFile(spm) error: %v", err)
	}

	output, err := executeCommand(NewRootCommand(), "clean", "caches", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"DerivedData and SwiftPM caches", "DerivedData", "SPM caches", "╭"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want to contain %q", output, want)
		}
	}
}

type forbiddenInput struct{}

func (forbiddenInput) Read([]byte) (int, error) { panic("read-only command attempted to read stdin") }

func TestReadOnlyDerivedNeverReadsInput(t *testing.T) {
	original := DerivedTarget
	defer func() { DerivedTarget = original }()
	DerivedTarget = internal.Target{Name: "DerivedData", Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(DerivedTarget.Path, "entry"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"clean", "derived", "--list"}, {"clean", "derived", "--dry-run"}, {"clean", "derived", "--select", "--list"}, {"clean", "derived", "--select", "--dry-run"}} {
		root := NewRootCommand()
		var out bytes.Buffer
		root.SetIn(forbiddenInput{})
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "Delete which") || strings.Contains(out.String(), "[y/N]") {
			t.Fatal(out.String())
		}
	}
	if _, err := os.Stat(filepath.Join(DerivedTarget.Path, "entry")); err != nil {
		t.Fatal(err)
	}
}

func TestAllContinuesAndReturnsScanErrors(t *testing.T) {
	derived, spm := DerivedTarget, SPMTarget
	defer func() { DerivedTarget = derived; SPMTarget = spm }()
	DerivedTarget = internal.Target{Name: "DerivedData", Path: filepath.Join(t.TempDir(), "missing")}
	SPMTarget = internal.Target{Name: "SPM caches", Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(SPMTarget.Path, "entry"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := executeCommand(NewRootCommand(), "clean", "caches", "--yes")
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error=%v", err)
	}
	if !strings.Contains(out, "Nuked estimated") {
		t.Fatal(out)
	}
	entries, err := os.ReadDir(SPMTarget.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("successful target was not cleaned")
	}
}

func TestPlainCleanupHasNoTerminalControls(t *testing.T) {
	original := SPMTarget
	defer func() { SPMTarget = original }()
	SPMTarget = internal.Target{Name: "SPM caches", Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(SPMTarget.Path, "entry"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := executeCommand(NewRootCommand(), "clean", "spm", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out, "\r\x1b") {
		t.Fatalf("terminal controls in output: %q", out)
	}
}

func TestRootFlagsAreIndependentAndInherited(t *testing.T) {
	original := SPMTarget
	defer func() { SPMTarget = original }()
	SPMTarget = internal.Target{Name: "SPM caches", Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(SPMTarget.Path, "entry"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	first := NewRootCommand()
	var out bytes.Buffer
	first.SetOut(&out)
	first.SetIn(forbiddenInput{})
	first.SetArgs([]string{"clean", "--dry-run", "spm"})
	if err := first.Execute(); err != nil {
		t.Fatal(err)
	}
	second := NewRootCommand()
	clean, _, err := second.Find([]string{"clean"})
	if err != nil {
		t.Fatal(err)
	}
	dry, _ := clean.PersistentFlags().GetBool("dry-run")
	if dry {
		t.Fatal("flag state leaked between roots")
	}
	// Prefix --yes must apply through inherited flags.
	second.SetOut(&out)
	second.SetIn(forbiddenInput{})
	second.SetArgs([]string{"clean", "--yes", "spm"})
	if err := second.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(SPMTarget.Path, "entry")); !os.IsNotExist(err) {
		t.Fatal("prefix yes did not delete", err)
	}
}

func TestCommandCancellationBeforeScan(t *testing.T) {
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(forbiddenInput{})
	root.SetArgs([]string{"clean", "caches", "--yes"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := root.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
}

func TestConfirmationReadFailurePreventsDeletion(t *testing.T) {
	original := SPMTarget
	defer func() { SPMTarget = original }()
	SPMTarget = internal.Target{Name: "SPM caches", Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(SPMTarget.Path, "entry"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(errorInput{})
	root.SetArgs([]string{"clean", "spm"})
	if err := root.Execute(); err == nil {
		t.Fatal("read failure ignored")
	}
	if _, err := os.Stat(filepath.Join(SPMTarget.Path, "entry")); err != nil {
		t.Fatal("deleted after failed confirmation", err)
	}
}

type errorInput struct{}

func (errorInput) Read([]byte) (int, error) { return 0, errors.New("input failed") }

func TestCancellationInterruptsConfirmation(t *testing.T) {
	original := SPMTarget
	defer func() { SPMTarget = original }()
	SPMTarget = internal.Target{Name: "SPM caches", Path: t.TempDir()}
	if err := os.WriteFile(filepath.Join(SPMTarget.Path, "entry"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{}, 1)
	root := NewRootCommand()
	root.SetIn(reader)
	root.SetOut(promptNotifier{ready})
	root.SetErr(io.Discard)
	root.SetArgs([]string{"clean", "spm"})
	result := make(chan error, 1)
	go func() { result <- root.ExecuteContext(ctx) }()
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("confirmation never appeared")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("confirmation read not canceled")
	}
	if _, err := os.Stat(filepath.Join(SPMTarget.Path, "entry")); err != nil {
		t.Fatal("deleted after canceled confirmation", err)
	}
}

type promptNotifier struct{ ready chan struct{} }

func (w promptNotifier) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "[y/N]") {
		select {
		case w.ready <- struct{}{}:
		default:
		}
	}
	return len(p), nil
}

func TestAllPartialDeletionContinuesWithoutRescanning(t *testing.T) {
	derived, spm := DerivedTarget, SPMTarget
	defer func() { DerivedTarget = derived; SPMTarget = spm }()
	DerivedTarget = internal.Target{Name: "DerivedData", Path: t.TempDir()}
	SPMTarget = internal.Target{Name: "SPM caches", Path: t.TempDir()}
	for _, name := range []string{"A", "B"} {
		path := filepath.Join(DerivedTarget.Path, name)
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "file"), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(SPMTarget.Path, "cache"), []byte("cache"), 0644); err != nil {
		t.Fatal(err)
	}
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"clean", "caches"})
	root.SetIn(&confirmationMutation{mutate: func() {
		path := filepath.Join(DerivedTarget.Path, "B")
		if err := os.Rename(path, path+"-unconfirmed"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
	}})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "changed since scan") {
		t.Fatalf("error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(DerivedTarget.Path, "A")); !os.IsNotExist(err) {
		t.Fatal("valid entry was not cleaned", err)
	}
	for _, name := range []string{"B", "B-unconfirmed"} {
		if _, err := os.Stat(filepath.Join(DerivedTarget.Path, name)); err != nil {
			t.Fatal("replacement or unconfirmed sibling removed", err)
		}
	}
	if _, err := os.Stat(filepath.Join(SPMTarget.Path, "cache")); !os.IsNotExist(err) {
		t.Fatal("independent target was not cleaned", err)
	}
	if !strings.Contains(out.String(), "Nuked estimated 1 B from DerivedData") {
		t.Fatal(out.String())
	}
}

type confirmationMutation struct {
	mutate func()
	read   bool
}

func (r *confirmationMutation) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	r.mutate()
	return copy(p, "y\n"), nil
}
