package cmd

import (
	"bufio"
	"fmt"
	"github.com/faizmokh/nuke/internal"
	"github.com/spf13/cobra"
	"os"
)

func newDerivedCommand() *cobra.Command {
	var all, list, interactive, current, buildOnly bool
	var project, olderThan string
	cmd := &cobra.Command{Use: "derived", Short: "Clean Xcode DerivedData", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		scanOptions := internal.ScanOptions{Project: project, OlderThan: olderThan, Activity: true, BuildOnly: buildOnly}
		if current {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			selected, err := internal.FindCurrentProject(cwd)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Current project: %s\n", selected)
			scanOptions.MatchEntry = internal.CurrentProjectMatcher(selected)
		}
		opts := commonOptions(cmd)
		shouldSelect := !opts.yes && (interactive || (!all && !current && project == "" && olderThan == ""))
		var selectEntries func(*bufio.Reader, internal.ScanPlan) (internal.ScanPlan, error)
		if shouldSelect {
			selectEntries = func(in *bufio.Reader, p internal.ScanPlan) (internal.ScanPlan, error) {
				if _, count := p.Summary(); count == 0 {
					return p.Select(nil), nil
				}
				selected, err := internal.InteractiveSelect(cmd.OutOrStdout(), in, p.Entries)
				return p.Select(selected), err
			}
		}
		target := derivedTargetFor(cmd)
		if buildOnly {
			target.Name = "DerivedData build outputs"
			fmt.Fprintln(cmd.OutOrStdout(), "Build-only: Products and Intermediates.noindex; indexes and package checkouts are preserved.")
		}
		return runCleanup(cmd, []internal.Target{target}, scanOptions, selectEntries, list)
	}}
	cmd.Flags().BoolVar(&all, "all", false, "skip entry selection; still ask for confirmation")
	cmd.Flags().StringVar(&project, "project", "", "delete entries matching project regex")
	cmd.Flags().StringVar(&olderThan, "older-than", "", "delete entries older than threshold (for example 30d or 2025-01-01)")
	cmd.Flags().BoolVar(&list, "list", false, "list DerivedData entries without deleting")
	cmd.Flags().BoolVar(&interactive, "select", false, "choose DerivedData entries after applying filters")
	cmd.Flags().BoolVar(&current, "current", false, "only clean DerivedData belonging to the current workspace/project")
	cmd.Flags().BoolVar(&buildOnly, "build-only", false, "remove only Build/Products and Build/Intermediates.noindex; preserve indexes and package checkouts")
	cmd.MarkFlagsMutuallyExclusive("current", "project")
	cmd.MarkFlagsMutuallyExclusive("current", "all")
	return cmd
}
