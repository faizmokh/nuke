package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var Version = "dev"
var DerivedTarget = internal.Target{Name: "DerivedData", Path: "~/Library/Developer/Xcode/DerivedData"}
var SPMTarget = internal.Target{Name: "SPM caches", Path: "~/Library/Caches/org.swift.swiftpm"}
var ArchivesTarget = internal.Target{Name: "Xcode Archives", Path: "~/Library/Developer/Xcode/Archives"}
var DeviceSupportTarget = internal.Target{Name: "iOS DeviceSupport", Path: "~/Library/Developer/Xcode/iOS DeviceSupport"}
var ModuleCacheTarget = internal.Target{Name: "Xcode module cache", Path: "~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex"}
var isInteractiveTerminal = tui.IsInteractiveTerminal
var runDerivedPicker = tui.RunDerivedPickerContext

// NewRootCommand creates independent commands and flag state for each invocation.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{Use: "nuke", Short: "Clean up Xcode and iOS development caches and prepare Swift package dependencies", Version: Version, SilenceUsage: true, SilenceErrors: true}
	root.Example = "  nuke clean                       # Choose a cleanup target\n  nuke clean derived --current     # Clean the current project\n  nuke clean caches --dry-run       # Preview DerivedData and SwiftPM\n  nuke spm download                # Prepare project dependencies\n  nuke status\n  nuke doctor"
	root.RunE = func(cmd *cobra.Command, args []string) error { return cmd.Help() }
	root.Args = cobra.NoArgs
	clean := newCleanCommand()
	root.AddCommand(clean, newStatusCommand(), newDoctorCommand(), newSPMCommand())
	root.PersistentFlags().String("derived-data", "", "use a custom DerivedData root directory")
	clean.AddCommand(newDerivedCommand(), newSimulatorsCommand())
	for _, spec := range []struct {
		name, help string
		target     *internal.Target
	}{
		{"spm", "Clean Swift Package Manager caches", &SPMTarget},
		{"archives", "Clean Xcode Archives", &ArchivesTarget},
		{"device-support", "Clean iOS DeviceSupport", &DeviceSupportTarget},
		{"module-cache", "Clean the Xcode module cache", &ModuleCacheTarget},
	} {
		targetCommand := &cobra.Command{Use: spec.name, Short: spec.help, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			target := *spec.target
			if spec.name == "module-cache" && cmd.Flags().Changed("derived-data") {
				target.Path = filepath.Join(derivedTargetFor(cmd).Path, "ModuleCache.noindex")
			}
			return runCleanup(cmd, []internal.Target{target}, internal.ScanOptions{}, nil, false)
		}}
		if spec.name == "archives" {
			targetCommand.Flags().Bool("trash", false, "move archives to Trash for recovery; space is reclaimed only after emptying Trash")
		}
		clean.AddCommand(targetCommand)
	}
	clean.AddCommand(&cobra.Command{Use: "caches", Short: "Clean DerivedData and SPM caches", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		return runCleanup(cmd, []internal.Target{derivedTargetFor(cmd), SPMTarget}, internal.ScanOptions{}, nil, false)
	}})
	return root
}

var runTargetMenu = tui.RunTargetMenu

func newCleanCommand() *cobra.Command {
	clean := &cobra.Command{Use: "clean [target]", Short: "Clean caches or choose a cleanup target", Args: cobra.NoArgs}
	clean.PersistentFlags().BoolP("yes", "y", false, "skip entry selection and confirmation")
	clean.PersistentFlags().Bool("dry-run", false, "preview without selecting or deleting")
	clean.RunE = func(cmd *cobra.Command, args []string) error {
		opts := commonOptions(cmd)
		if opts.yes || opts.dryRun || !isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
			return fmt.Errorf("choose an explicit cleanup target, for example: nuke clean derived or nuke clean caches --dry-run (see nuke clean --help)")
		}
		choices := []tui.TargetChoice{
			{Name: "derived", Label: "DerivedData", Description: "Choose Xcode project caches"},
			{Name: "caches", Label: "Common caches", Description: "All DerivedData and SwiftPM caches"},
			{Name: "spm", Label: "SwiftPM", Description: "Swift Package Manager caches"},
			{Name: "archives", Label: "Archives", Description: "Xcode build archives"},
			{Name: "device-support", Label: "Device support", Description: "Cached iOS device support files"},
			{Name: "module-cache", Label: "Module cache", Description: "Compiled Xcode modules"},
			{Name: "simulators", Label: "Unavailable simulators", Description: "Only unavailable CoreSimulator devices"},
		}
		name, err := runTargetMenu(cmd.Context(), cmd.OutOrStdout(), cmd.InOrStdin(), choices)
		if err != nil || name == "" {
			return err
		}
		target, _, err := cmd.Find([]string{name})
		if err != nil {
			return err
		}
		if target == cmd || target.RunE == nil {
			return fmt.Errorf("unknown cleanup target %q", name)
		}
		// Cobra normally assigns context when executing a child command.
		target.SetContext(cmd.Context())
		// Initialize inherited flags before directly invoking the same handler.
		if err := target.ParseFlags(nil); err != nil {
			return err
		}
		return target.RunE(target, nil)
	}
	return clean
}

func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return NewRootCommand().ExecuteContext(ctx)
}

func derivedTargetFor(cmd *cobra.Command) internal.Target {
	target := DerivedTarget
	path, _ := cmd.Flags().GetString("derived-data")
	if path != "" {
		target.Path = internal.ExpandHome(path)
	}
	return target
}
