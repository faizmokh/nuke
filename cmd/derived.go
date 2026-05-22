package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var isInteractiveTerminal = tui.IsInteractiveTerminal
var runDerivedPicker = tui.RunDerivedPicker

var (
	derivedAllFlag         bool
	derivedProjectFlag     string
	derivedOlderThanFlag   string
	derivedListFlag        bool
	derivedInteractiveFlag bool
)

var derivedCmd = &cobra.Command{
	Use:   "derived",
	Short: "Clean Xcode DerivedData",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDerived()
	},
}

func init() {
	bindFlags(derivedCmd)
	derivedCmd.Flags().BoolVar(&derivedAllFlag, "all", false, "delete all DerivedData")
	derivedCmd.Flags().StringVar(&derivedProjectFlag, "project", "", "delete entries matching project regex")
	derivedCmd.Flags().StringVar(&derivedOlderThanFlag, "older-than", "", "delete entries older than threshold (for example 30d or 2025-01-01)")
	derivedCmd.Flags().BoolVar(&derivedListFlag, "list", false, "list DerivedData entries without deleting")
	derivedCmd.Flags().BoolVar(&derivedInteractiveFlag, "interactive", false, "choose DerivedData entries interactively")
	rootCmd.AddCommand(derivedCmd)
}

func runDerived() error {
	out := rootCmd.OutOrStdout()
	in := rootCmd.InOrStdin()
	reader := bufio.NewReader(in)
	interactiveDefault := !yesFlag && !derivedAllFlag && !derivedInteractiveFlag && derivedProjectFlag == "" && derivedOlderThanFlag == ""
	shouldUsePicker := isInteractiveTerminal(in, out) && !dryRunFlag && !derivedListFlag && !yesFlag && (derivedInteractiveFlag || interactiveDefault)
	if shouldUsePicker {
		entries, err := os.ReadDir(internal.ExpandHome(DerivedTarget.Path))
		if err != nil {
			return fmt.Errorf("scanning %s: %w", DerivedTarget.Name, err)
		}
		if len(entries) == 0 {
			fmt.Fprintf(out, "%s: nothing to clean\n", DerivedTarget.Name)
			return nil
		}

		selected, err := runDerivedPicker(out, in, DerivedTarget, derivedProjectFlag, derivedOlderThanFlag)
		if err != nil {
			if errors.Is(err, tui.ErrNoEntries) {
				fmt.Fprintf(out, "%s: nothing to clean\n", DerivedTarget.Name)
				return nil
			}
			return err
		}
		if len(selected) == 0 {
			fmt.Fprintln(out, "No items selected.")
			return nil
		}

		bytes, items := internal.EntriesSummary(selected)
		fmt.Fprintf(out, "%s: %s in %d items\n", DerivedTarget.Name, internal.HumanSize(bytes), items)
		fmt.Fprintf(out, "Nuke %s? [y/N] ", internal.HumanSize(bytes))
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)
		if response != "y" && response != "Y" && response != "yes" {
			return nil
		}

		bar := internal.NewProgressBar(out, items)
		freed, err := internal.NukeEntries(selected, func(current, total int) {
			bar.Update(current)
		})
		bar.Done()
		if err != nil {
			return err
		}

		fmt.Fprintf(out, "Nuked %s from %s\n", internal.HumanSize(freed), DerivedTarget.Name)
		return nil
	}

	entries, err := internal.ScanDerived(DerivedTarget)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintf(out, "%s: nothing to clean\n", DerivedTarget.Name)
		return nil
	}

	if derivedProjectFlag != "" {
		entries, err = internal.FilterByProject(entries, derivedProjectFlag)
		if err != nil {
			return err
		}
	}

	if derivedOlderThanFlag != "" {
		threshold, err := internal.ParseAgeThreshold(derivedOlderThanFlag)
		if err != nil {
			return err
		}
		entries = internal.FilterByAge(entries, threshold)
	}

	if len(entries) == 0 {
		fmt.Fprintf(out, "%s: nothing to clean\n", DerivedTarget.Name)
		return nil
	}

	if derivedInteractiveFlag || interactiveDefault {
		entries, err = internal.InteractiveSelect(out, reader, entries)
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Fprintln(out, "No items selected.")
			return nil
		}
	}

	bytes, items := internal.EntriesSummary(entries)
	if derivedListFlag || dryRunFlag {
		internal.FormatEntriesTable(out, entries)
		fmt.Fprintf(out, "\n%s: %s in %d items\n", DerivedTarget.Name, internal.HumanSize(bytes), items)
		return nil
	}

	fmt.Fprintf(out, "%s: %s in %d items\n", DerivedTarget.Name, internal.HumanSize(bytes), items)
	if !yesFlag {
		fmt.Fprintf(out, "Nuke %s? [y/N] ", internal.HumanSize(bytes))
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)
		if response != "y" && response != "Y" && response != "yes" {
			return nil
		}
	}

	bar := internal.NewProgressBar(out, items)
	freed, err := internal.NukeEntries(entries, func(current, total int) {
		bar.Update(current)
	})
	bar.Done()
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Nuked %s from %s\n", internal.HumanSize(freed), DerivedTarget.Name)
	return nil
}
