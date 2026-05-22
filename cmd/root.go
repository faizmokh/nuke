package cmd

import (
	"fmt"
	"strings"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var Version = "dev"

var (
	yesFlag    bool
	dryRunFlag bool
)

var DerivedTarget = internal.Target{
	Name: "DerivedData",
	Path: "~/Library/Developer/Xcode/DerivedData",
}

var SPMTarget = internal.Target{
	Name: "SPM caches",
	Path: "~/Library/Caches/org.swift.swiftpm",
}

var ArchivesTarget = internal.Target{
	Name: "Xcode Archives",
	Path: "~/Library/Developer/Xcode/Archives",
}

var DeviceSupportTarget = internal.Target{
	Name: "iOS DeviceSupport",
	Path: "~/Library/Developer/Xcode/iOS DeviceSupport",
}

var ModuleCacheTarget = internal.Target{
	Name: "Xcode module cache",
	Path: "~/Library/Developer/Xcode/DerivedData/ModuleCache.noindex",
}

var rootCmd = &cobra.Command{
	Use:     "nuke",
	Short:   "Clean up Xcode and iOS development caches",
	Version: Version,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&yesFlag, "yes", "y", false, "skip confirmation prompt")
	rootCmd.PersistentFlags().BoolVar(&dryRunFlag, "dry-run", false, "show what would be deleted without deleting")
}

func bindFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolVar(&dryRunFlag, "dry-run", false, "show what would be deleted without deleting")
}

func runTarget(target internal.Target) error {
	out := rootCmd.OutOrStdout()
	in := rootCmd.InOrStdin()
	interactive := isInteractiveTerminal(in, out)

	bytes, items, err := internal.Scan(target)
	if err != nil {
		return err
	}
	if items == 0 {
		fmt.Fprintf(out, "%s: nothing to clean\n", target.Name)
		return nil
	}

	if interactive {
		fmt.Fprintln(out, tui.RenderSummaryCard(target.Name, fmt.Sprintf("Reclaimable: %s", internal.HumanSize(bytes)), fmt.Sprintf("Items: %d", items)))
	} else {
		fmt.Fprintf(out, "%s: %s in %d items\n", target.Name, internal.HumanSize(bytes), items)
	}

	if dryRunFlag {
		return nil
	}

	if !yesFlag {
		confirmed, err := confirmCleanup(in, out, interactive, fmt.Sprintf("Nuke %s? [y/N]", internal.HumanSize(bytes)))
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	progressBar := newProgressBar(out, interactive, target.Name, items)
	freed, err := internal.Nuke(target, func(current, total int) {
		progressBar.Update(current)
	})
	progressBar.Done()
	if err != nil {
		return err
	}

	if interactive {
		fmt.Fprintln(out, tui.RenderSummaryCard("Cleanup Complete", fmt.Sprintf("Target: %s", target.Name), fmt.Sprintf("Freed: %s", internal.HumanSize(freed))))
	} else {
		fmt.Fprintf(out, "Nuked %s from %s\n", internal.HumanSize(freed), target.Name)
	}

	return nil
}

func runAll() error {
	out := rootCmd.OutOrStdout()
	in := rootCmd.InOrStdin()
	interactive := isInteractiveTerminal(in, out)
	targets := []internal.Target{DerivedTarget, SPMTarget}
	any := false
	lines := make([]string, 0, len(targets))

	for _, t := range targets {
		bytes, items, err := internal.Scan(t)
		if err != nil {
			fmt.Fprintf(out, "%s: %v\n", t.Name, err)
			continue
		}
		if items > 0 {
			if interactive {
				lines = append(lines, fmt.Sprintf("%s: %s in %d items", t.Name, internal.HumanSize(bytes), items))
			} else {
				fmt.Fprintf(out, "%s: %s in %d items\n", t.Name, internal.HumanSize(bytes), items)
			}
			any = true
		}
	}

	if !any {
		fmt.Fprintln(out, "Nothing to clean.")
		return nil
	}
	if interactive {
		fmt.Fprintln(out, tui.RenderSummaryCard("All Targets", lines...))
	}

	if dryRunFlag {
		return nil
	}

	if !yesFlag {
		confirmed, err := confirmCleanup(in, out, interactive, "Nuke all? [y/N]")
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}

	for _, t := range targets {
		_, items, err := internal.Scan(t)
		if err != nil {
			fmt.Fprintf(out, "%s: %v\n", t.Name, err)
			continue
		}

		bar := newProgressBar(out, interactive, t.Name, items)
		freed, err := internal.Nuke(t, func(current, total int) {
			bar.Update(current)
		})
		bar.Done()
		if err != nil {
			fmt.Fprintf(out, "%s: %v\n", t.Name, err)
			continue
		}
		if freed > 0 {
			if interactive {
				fmt.Fprintln(out, tui.RenderSummaryCard("Cleanup Complete", fmt.Sprintf("Target: %s", t.Name), fmt.Sprintf("Freed: %s", internal.HumanSize(freed))))
			} else {
				fmt.Fprintf(out, "Nuked %s from %s\n", internal.HumanSize(freed), t.Name)
			}
		}
	}

	return nil
}

type progressBar interface {
	Update(current int)
	Done()
}

func newProgressBar(out interface{ Write([]byte) (int, error) }, interactive bool, label string, total int) progressBar {
	if interactive {
		return tui.NewInlineProgress(out, label, total)
	}
	return internal.NewProgressBar(out, total)
}

func confirmCleanup(in interface{ Read([]byte) (int, error) }, out interface{ Write([]byte) (int, error) }, interactive bool, prompt string) (bool, error) {
	var response string
	if interactive {
		fmt.Fprintln(out, tui.RenderConfirmPrompt(prompt))
	} else {
		fmt.Fprintf(out, "%s ", prompt)
	}
	_, err := fmt.Fscanln(in, &response)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return false, err
	}
	return response == "y" || response == "Y" || response == "yes", nil
}
