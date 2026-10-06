package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Show cache sizes and unavailable simulator counts without deleting",
		Long: "Show estimated sizes and top-level item counts for all filesystem cleanup targets, plus unavailable simulator counts. Missing cache directories are normal. Nothing is deleted and no confirmation is requested.",
		Args: cobra.NoArgs, RunE: runStatus,
	}
}

type statusRow struct{ name, size, items, state string }

func runStatus(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	derivedTarget := derivedTargetFor(cmd)
	moduleTarget := ModuleCacheTarget
	if cmd.Flags().Changed("derived-data") {
		moduleTarget.Path = filepath.Join(derivedTarget.Path, "ModuleCache.noindex")
	}
	targets := []internal.Target{derivedTarget, SPMTarget, ArchivesTarget, DeviceSupportTarget, moduleTarget}
	rows := make([]statusRow, 0, len(targets)+1)
	var errs []error
	var derived internal.ScanPlan
	var total int64
	partial := false
	for i, target := range targets {
		if err := cmd.Context().Err(); err != nil {
			return err
		}
		row := statusRow{name: target.Name, size: "0 B", items: "0", state: "empty"}
		path, err := filepath.EvalSymlinks(internal.ExpandHome(target.Path))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				row.state = "not present"
			} else {
				row.size = "—"
				row.items = "—"
				row.state = "unavailable"
				partial = true
				errs = append(errs, fmt.Errorf("%s: %w", target.Name, err))
			}
			rows = append(rows, row)
			continue
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		// ModuleCache is normally an immediate child of DerivedData. Reuse its
		// measured size and only enumerate immediate children for the item count.
		// A symlink entry wasn't traversed by the parent scan and must scan separately.
		included := false
		var cached *internal.DerivedEntry
		if i == len(targets)-1 {
			for _, entry := range derived.Entries {
				if entry.Path == path {
					info, statErr := os.Lstat(path)
					if statErr == nil && info.IsDir() {
						copy := entry
						cached = &copy
						included = true
					}
					break
				}
			}
		}
		var size int64
		var count int
		if cached != nil {
			size = cached.Size
			err = cached.Err
			if err == nil {
				var entries []os.DirEntry
				entries, err = os.ReadDir(path)
				count = len(entries)
			}
		} else {
			var plan internal.ScanPlan
			plan, err = internal.ScanTarget(cmd.Context(), target, internal.ScanOptions{}, nil)
			size, count = plan.Summary()
			if i == 0 {
				derived = plan
			}
		}
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		row.size = internal.HumanSize(size)
		row.items = fmt.Sprint(count)
		if count > 0 {
			row.state = "present"
		}
		if err != nil {
			row.state = "partial / unavailable"
			partial = true
			errs = append(errs, fmt.Errorf("%s: %w", target.Name, err))
			if cached != nil {
				row.size = "—"
				row.items = "—"
			}
		}
		if included {
			row.state += "; included in DerivedData"
		} else {
			total += size
		}
		rows = append(rows, row)
	}
	devices, err := listUnavailableSimulators(cmd.Context())
	if cmd.Context().Err() != nil {
		return cmd.Context().Err()
	}
	simulatorRow := statusRow{name: "Unavailable simulators", size: "—", items: fmt.Sprint(len(devices)), state: "device count; size not measured"}
	if err != nil {
		simulatorRow.items = "—"
		simulatorRow.state = "unavailable"
		errs = append(errs, fmt.Errorf("Simulators: %w", err))
	}
	rows = append(rows, simulatorRow)
	totalLabel := "Total estimated filesystem size"
	if partial {
		totalLabel = "Known estimated filesystem size (partial)"
	}
	note := "Sizes are logical estimates. Items are top-level entries; simulator items are devices."
	totalLine := fmt.Sprintf("%s: %s", totalLabel, internal.HumanSize(total))
	if isInteractiveTerminal(cmd.InOrStdin(), out) {
		lines := make([]string, 0, len(rows)+3)
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("%s: %s · %s items · %s", row.name, row.size, row.items, row.state))
		}
		lines = append(lines, "", totalLine, note)
		fmt.Fprintln(out, tui.RenderSummaryCardFor(out, "Cache Status", lines...))
	} else {
		table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "Target\tEstimated size\tItems\tStatus")
		for _, row := range rows {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", row.name, row.size, row.items, row.state)
		}
		if err := table.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(out, "\n%s\n%s\n", totalLine, note)
	}
	for _, err := range errs {
		fmt.Fprintln(cmd.ErrOrStderr(), strings.TrimSpace(err.Error()))
	}
	return errors.Join(errs...)
}
