package cmd

import (
	"bufio"
	"context"
	"fmt"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var listUnavailableSimulators = func(ctx context.Context) ([]internal.SimulatorDevice, error) {
	return internal.ListUnavailableSimulators(func(args ...string) ([]byte, error) { return internal.RunSimctlContext(ctx, args...) })
}

var deleteUnavailableSimulators = func(ctx context.Context) error {
	return internal.DeleteUnavailableSimulators(func(args ...string) ([]byte, error) { return internal.RunSimctlContext(ctx, args...) })
}

func newSimulatorsCommand() *cobra.Command {
	return &cobra.Command{Use: "simulators", Short: "Clean unavailable simulators", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error { return runSimulators(cmd) }}
}

func runSimulators(cmd *cobra.Command) error {
	opts := commonOptions(cmd)
	out := cmd.OutOrStdout()
	in := cmd.InOrStdin()
	interactive := isInteractiveTerminal(in, out)

	devices, err := listUnavailableSimulators(cmd.Context())
	if err != nil {
		return fmt.Errorf("listing unavailable simulators: %w", err)
	}
	if len(devices) == 0 {
		fmt.Fprintln(out, "Simulators: nothing to clean")
		return nil
	}

	count := len(devices)
	if interactive {
		lines := make([]string, 0, len(devices)+1)
		for _, device := range devices {
			lines = append(lines, fmt.Sprintf("%s (%s)", device.Name, device.Runtime))
		}
		lines = append(lines, fmt.Sprintf("%d unavailable %s", count, pluralize(count, "simulator", "simulators")))
		fmt.Fprintln(out, tui.RenderSummaryCardFor(out, "Unavailable Simulators", lines...))
	} else {
		fmt.Fprintln(out, "Unavailable simulators:")
		for _, device := range devices {
			fmt.Fprintf(out, "- %s (%s)\n", device.Name, device.Runtime)
		}
		fmt.Fprintf(out, "\n%d unavailable %s\n", count, pluralize(count, "simulator", "simulators"))
	}

	if opts.dryRun {
		return nil
	}

	if !opts.yes {
		prompt := fmt.Sprintf("Delete %d unavailable %s? [y/N]", count, pluralize(count, "simulator", "simulators"))
		input, closeInput, err := cancellableInput(cmd.Context(), in)
		if err != nil {
			return err
		}
		defer closeInput()
		confirmed, err := confirmCleanup(bufio.NewReader(input), out, interactive, prompt)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}

	}

	if err := deleteUnavailableSimulators(cmd.Context()); err != nil {
		return fmt.Errorf("deleting unavailable simulators: %w", err)
	}

	if interactive {
		fmt.Fprintln(out, tui.RenderSummaryCardFor(out, "Cleanup Complete", fmt.Sprintf("Deleted %d unavailable %s", count, pluralize(count, "simulator", "simulators"))))
	} else {
		fmt.Fprintf(out, "Deleted %d unavailable %s\n", count, pluralize(count, "simulator", "simulators"))
	}
	return nil
}

func pluralize(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
