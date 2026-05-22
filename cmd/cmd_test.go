package cmd

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
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
	resetHelpFlag(archivesCmd)
	resetHelpFlag(deviceSupportCmd)
	resetHelpFlag(moduleCacheCmd)
	resetHelpFlag(simulatorsCmd)
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

func TestArchivesCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"archives"})
	if err != nil {
		t.Error("archives subcommand not registered")
	}
}

func TestDeviceSupportCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"device-support"})
	if err != nil {
		t.Error("device-support subcommand not registered")
	}
}

func TestModuleCacheCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"module-cache"})
	if err != nil {
		t.Error("module-cache subcommand not registered")
	}
}

func TestSimulatorsCommandRegistered(t *testing.T) {
	_, _, err := rootCmd.Find([]string{"simulators"})
	if err != nil {
		t.Error("simulators subcommand not registered")
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

func TestRootHelpListsExpandedCleanupTargets(t *testing.T) {
	output, err := executeCommand(rootCmd, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"archives", "device-support", "module-cache"} {
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
	runDerivedPicker = func(out io.Writer, in io.Reader, target internal.Target, projectPattern string, olderThan string) ([]internal.DerivedEntry, error) {
		pickerCalled = true
		return nil, nil
	}

	_, err := executeCommand(rootCmd, "derived")
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
	runDerivedPicker = func(out io.Writer, in io.Reader, target internal.Target, projectPattern string, olderThan string) ([]internal.DerivedEntry, error) {
		pickerCalled = true
		return nil, nil
	}

	if _, err := executeCommand(rootCmd, "derived", "--list"); err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}
	if pickerCalled {
		t.Fatal("expected --list to bypass picker")
	}

	pickerCalled = false
	if _, err := executeCommand(rootCmd, "derived", "--yes"); err != nil {
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
	runDerivedPicker = func(out io.Writer, in io.Reader, target internal.Target, projectPattern string, olderThan string) ([]internal.DerivedEntry, error) {
		return nil, tui.ErrNoEntries
	}

	output, err := executeCommand(rootCmd, "derived", "--interactive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "nothing to clean") {
		t.Fatalf("output = %q, want nothing to clean message", output)
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

func TestArchivesHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "archives", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Archives") {
		t.Errorf("help output = %q, want to contain 'Archives'", output)
	}
}

func TestDeviceSupportHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "device-support", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "DeviceSupport") {
		t.Errorf("help output = %q, want to contain 'DeviceSupport'", output)
	}
}

func TestModuleCacheHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "module-cache", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "module cache") {
		t.Errorf("help output = %q, want to contain 'module cache'", output)
	}
}

func TestSimulatorsHelp(t *testing.T) {
	output, err := executeCommand(rootCmd, "simulators", "--help")
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

	_, err := executeCommand(rootCmd, "archives", "--yes")
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

	output, err := executeCommand(rootCmd, "archives", "--dry-run")
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

	_, err := executeCommand(rootCmd, "device-support", "--yes")
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

	_, err := executeCommand(rootCmd, "module-cache", "--yes")
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
	listUnavailableSimulators = func() ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{
			{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"},
			{Name: "iPad Pro", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-16-4", UDID: "DEF"},
		}, nil
	}
	deleteUnavailableSimulators = func() error {
		deleteCalled = true
		return nil
	}

	output, err := executeCommand(rootCmd, "simulators", "--dry-run")
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
	listUnavailableSimulators = func() ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"}}, nil
	}
	deleteUnavailableSimulators = func() error {
		deleteCalled = true
		return nil
	}

	output, err := executeCommand(rootCmd, "simulators", "--yes")
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
	listUnavailableSimulators = func() ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"}}, nil
	}
	deleteUnavailableSimulators = func() error {
		deleteCalled = true
		return nil
	}

	output, err := executeCommand(rootCmd, "simulators", "--dry-run")
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

	listUnavailableSimulators = func() ([]internal.SimulatorDevice, error) {
		return nil, errors.New("simctl exploded")
	}

	_, err := executeCommand(rootCmd, "simulators")
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

	listUnavailableSimulators = func() ([]internal.SimulatorDevice, error) {
		return []internal.SimulatorDevice{{Name: "iPhone 8", Runtime: "com.apple.CoreSimulator.SimRuntime.iOS-15-5", UDID: "ABC"}}, nil
	}
	deleteUnavailableSimulators = func() error {
		return errors.New("permission denied")
	}

	_, err := executeCommand(rootCmd, "simulators", "--yes")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "deleting unavailable simulators") {
		t.Fatalf("error = %q, want delete context", err)
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

	output, err := executeCommand(rootCmd, "all", "--dry-run")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"All Targets", "DerivedData", "SPM caches", "╭"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want to contain %q", output, want)
		}
	}
}
