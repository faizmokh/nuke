package cmd

import (
	"fmt"

	"github.com/faizmokh/nuke/internal"
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

var rootCmd = &cobra.Command{
	Use:     "nuke",
	Short:   "Clean up Xcode DerivedData and SPM caches",
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
	return internal.Run(rootCmd.OutOrStdout(), rootCmd.InOrStdin(), target, yesFlag, dryRunFlag)
}

func runAll() error {
	targets := []internal.Target{DerivedTarget, SPMTarget}
	any := false

	for _, t := range targets {
		bytes, items, err := internal.Scan(t)
		if err != nil {
			fmt.Fprintf(rootCmd.OutOrStdout(), "%s: %v\n", t.Name, err)
			continue
		}
		if items > 0 {
			fmt.Fprintf(rootCmd.OutOrStdout(), "%s: %s in %d items\n", t.Name, internal.HumanSize(bytes), items)
			any = true
		}
	}

	if !any {
		fmt.Fprintln(rootCmd.OutOrStdout(), "Nothing to clean.")
		return nil
	}

	if dryRunFlag {
		return nil
	}

	if !yesFlag {
		fmt.Fprintf(rootCmd.OutOrStdout(), "\nNuke all? [y/N] ")
		var response string
		fmt.Fscanln(rootCmd.InOrStdin(), &response)
		if response != "y" && response != "Y" && response != "yes" {
			return nil
		}
	}

	for _, t := range targets {
		freed, err := internal.Nuke(t)
		if err != nil {
			fmt.Fprintf(rootCmd.OutOrStdout(), "%s: %v\n", t.Name, err)
			continue
		}
		if freed > 0 {
			fmt.Fprintf(rootCmd.OutOrStdout(), "Nuked %s from %s\n", internal.HumanSize(freed), t.Name)
		}
	}

	return nil
}
