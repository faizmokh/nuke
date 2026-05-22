package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var listUnavailableSimulators = func() ([]internal.SimulatorDevice, error) {
	return internal.ListUnavailableSimulators(internal.RunSimctl)
}

var deleteUnavailableSimulators = func() error {
	return internal.DeleteUnavailableSimulators(internal.RunSimctl)
}

var simulatorsCmd = &cobra.Command{
	Use:   "simulators",
	Short: "Clean unavailable simulators",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSimulators()
	},
}

func init() {
	bindFlags(simulatorsCmd)
	rootCmd.AddCommand(simulatorsCmd)
}

func runSimulators() error {
	out := rootCmd.OutOrStdout()
	in := rootCmd.InOrStdin()
	interactive := isInteractiveTerminal(in, out)

	devices, err := listUnavailableSimulators()
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
		fmt.Fprintln(out, tui.RenderSummaryCard("Unavailable Simulators", lines...))
	} else {
		fmt.Fprintln(out, "Unavailable simulators:")
		for _, device := range devices {
			fmt.Fprintf(out, "- %s (%s)\n", device.Name, device.Runtime)
		}
		fmt.Fprintf(out, "\n%d unavailable %s\n", count, pluralize(count, "simulator", "simulators"))
	}

	if dryRunFlag {
		return nil
	}

	if !yesFlag {
		prompt := fmt.Sprintf("Delete %d unavailable %s? [y/N]", count, pluralize(count, "simulator", "simulators"))
		if interactive {
			fmt.Fprintln(out, tui.RenderConfirmPrompt(prompt))
		} else {
			fmt.Fprintf(out, "%s ", prompt)
		}
		reader := bufio.NewReader(in)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(response)
		if response != "y" && response != "Y" && response != "yes" {
			return nil
		}
	}

	if err := deleteUnavailableSimulators(); err != nil {
		return fmt.Errorf("deleting unavailable simulators: %w", err)
	}

	if interactive {
		fmt.Fprintln(out, tui.RenderSummaryCard("Cleanup Complete", fmt.Sprintf("Deleted %d unavailable %s", count, pluralize(count, "simulator", "simulators"))))
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
