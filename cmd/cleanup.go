package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/muesli/cancelreader"
	"github.com/spf13/cobra"
)

type cleanupOptions struct{ yes, dryRun bool }

func commonOptions(cmd *cobra.Command) cleanupOptions {
	yes, _ := cmd.Flags().GetBool("yes")
	dry, _ := cmd.Flags().GetBool("dry-run")
	return cleanupOptions{yes, dry}
}

type progressBar interface {
	Update(int)
	Done()
}
type quietProgress struct{}

func (quietProgress) Update(int) {}
func (quietProgress) Done()      {}
func newProgressBar(out io.Writer, interactive bool, label string, total int) progressBar {
	if interactive {
		return tui.NewInlineProgress(out, label, total)
	}
	return quietProgress{}
}

func confirmCleanup(in *bufio.Reader, out io.Writer, interactive bool, prompt string) (bool, error) {
	if interactive {
		fmt.Fprintln(out, tui.RenderConfirmPromptFor(out, prompt))
	} else {
		fmt.Fprintf(out, "%s ", prompt)
	}
	return internal.ReadConfirmation(in)
}

// runCleanup owns scan -> selection -> confirmation -> deletion -> reporting for
// every filesystem command. A picker supplies its scan plan rather than rescanning.
func runCleanup(cmd *cobra.Command, targets []internal.Target, scanOptions internal.ScanOptions, selectEntries func(*bufio.Reader, internal.ScanPlan) (internal.ScanPlan, error), list bool) error {
	opts := commonOptions(cmd)
	trash, _ := cmd.Flags().GetBool("trash")
	out, in := cmd.OutOrStdout(), cmd.InOrStdin()
	if trash {
		fmt.Fprintln(out, "Move to Trash: archives remain recoverable in Trash. Disk space is reclaimed only after emptying Trash.")
	}
	reader := bufio.NewReader(in)
	if !opts.yes && !opts.dryRun && !list {
		input, closeInput, err := cancellableInput(cmd.Context(), in)
		if err != nil {
			return err
		}
		defer closeInput()
		reader = bufio.NewReader(input)
	}
	interactive := isInteractiveTerminal(in, out)
	var plans []internal.ScanPlan
	var errs []error
	for _, target := range targets {
		var plan internal.ScanPlan
		var err error
		noEntries := false
		usePicker := selectEntries != nil && interactive && !opts.yes && !opts.dryRun && !list
		if usePicker {
			plan, err = runDerivedPicker(cmd.Context(), out, in, target, scanOptions)
			if errors.Is(err, tui.ErrNoEntries) {
				noEntries = true
				err = nil
			}
		} else {
			plan, err = internal.ScanTarget(cmd.Context(), target, scanOptions, nil)
		}
		scanErr := err
		if err != nil {
			errs = append(errs, err)
		}
		if cmd.Context().Err() != nil {
			return errors.Join(append(errs, cmd.Context().Err())...)
		}
		for _, entry := range plan.Entries {
			if entry.Err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: unavailable: %v\n", entry.Name, entry.Err)
			}
		}
		_, available := plan.Summary()
		if selectEntries != nil && !usePicker && !opts.dryRun && !list {
			plan, err = selectEntries(reader, plan)
			if err != nil {
				return errors.Join(append(errs, err, cmd.Context().Err())...)
			}
		}
		size, count := plan.Summary()
		if count == 0 {
			if scanErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %v\n", target.Name, scanErr)
				continue
			}
			if !opts.dryRun && !list && selectEntries != nil && ((usePicker && !noEntries) || available > 0) {
				fmt.Fprintln(out, "No items selected.")
			} else {
				fmt.Fprintf(out, "%s: nothing to clean\n", target.Name)
			}
			continue
		}
		if list || (opts.dryRun && (scanOptions.Activity || trash)) {
			internal.FormatEntriesTable(out, plan.Entries)
		}
		plans = append(plans, plan)
		if !interactive {
			fmt.Fprintf(out, "%s: %s in %d items (estimated)\n", target.Name, internal.HumanSize(size), count)
		}
	}
	if interactive && len(plans) > 0 {
		title := targets[0].Name
		if len(targets) > 1 {
			title = "DerivedData and SwiftPM caches"
		}
		var lines []string
		for _, p := range plans {
			size, count := p.Summary()
			label := "Reclaimable"
			if trash {
				label = "To move to Trash"
			}
			lines = append(lines, fmt.Sprintf("%s: %s: %s in %d items (estimated)", p.Target.Name, label, internal.HumanSize(size), count))
		}
		fmt.Fprintln(out, tui.RenderSummaryCardFor(out, title, lines...))
	}
	if len(plans) == 0 || opts.dryRun || list {
		return errors.Join(errs...)
	}
	if !opts.yes {
		var size int64
		var count int
		for _, p := range plans {
			s, n := p.Summary()
			size += s
			count += n
		}
		scope := targets[0].Name
		if len(targets) > 1 {
			scope = "DerivedData and SPM caches"
		}
		action := "Nuke"
		if trash {
			action = "Move to Trash"
		}
		confirmed, err := confirmCleanup(reader, out, interactive, fmt.Sprintf("%s %s: estimated %s in %d items? [y/N]", action, scope, internal.HumanSize(size), count))
		if err != nil {
			errs = append(errs, err)
		}
		if err != nil || !confirmed {
			return errors.Join(append(errs, cmd.Context().Err())...)
		}
	}
	for _, p := range plans {
		_, count := p.Summary()
		bar := newProgressBar(out, interactive, p.Target.Name, count)
		perform := internal.DeletePlan
		if trash {
			perform = internal.TrashPlan
		}
		result, err := perform(cmd.Context(), p.Select(p.Entries), func(current, total int) { bar.Update(current) })
		bar.Done()
		if err != nil {
			errs = append(errs, err)
		}
		for _, entry := range result.Entries {
			if entry.Err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Skipped %s: %v\n", entry.Entry.Name, entry.Err)
			}
		}
		if interactive {
			title := "Cleanup Complete"
			if err != nil {
				title = "Cleanup Partial"
			}
			itemLabel, sizeLabel := "Removed", "Freed (estimated)"
			if trash {
				itemLabel, sizeLabel = "Moved to Trash", "Size moved (estimated)"
			}
			fmt.Fprintln(out, tui.RenderSummaryCardFor(out, title, fmt.Sprintf("Target: %s", p.Target.Name), fmt.Sprintf("%s: %d items", itemLabel, result.Items), fmt.Sprintf("%s: %s", sizeLabel, internal.HumanSize(result.Bytes))))
		} else {
			if trash {
				fmt.Fprintf(out, "Moved estimated %s from %s to Trash (%d items). Empty Trash to reclaim disk space.\n", internal.HumanSize(result.Bytes), p.Target.Name, result.Items)
			} else {
				fmt.Fprintf(out, "Nuked estimated %s from %s (%d items)\n", internal.HumanSize(result.Bytes), p.Target.Name, result.Items)
			}
		}
		if cmd.Context().Err() != nil {
			errs = append(errs, cmd.Context().Err())
			break
		}
	}
	return errors.Join(errs...)
}

// Plain prompts use cancellable file reads so Ctrl-C can interrupt stdin as well
// as the scan. The original input file is retained for terminal capability checks
// and raw-mode setup in the picker.
func cancellableInput(ctx context.Context, in io.Reader) (io.Reader, func(), error) {
	r, err := cancelreader.NewReader(in)
	if err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { defer close(done); r.Cancel() })
	cleanup := func() {
		if !stop() {
			<-done
		}
		r.Close()
	}
	return contextInput{ctx: ctx, reader: r}, cleanup, nil
}

type contextInput struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextInput) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if r.ctx.Err() != nil {
		return 0, r.ctx.Err()
	}
	return n, err
}
