package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/faizmokh/nuke/internal"
	"github.com/faizmokh/nuke/internal/tui"
	"github.com/spf13/cobra"
)

var runDoctorChecks = internal.Doctor

func newDoctorCommand() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Diagnose Xcode, iOS SDKs, simulators, cache access, and disk space", Long: "Run read-only iOS development environment checks with actionable findings. Never installs tools, accepts licenses, writes probe files, or deletes caches. Errors return exit code 1; warnings alone return 0.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		derived := derivedTargetFor(cmd)
		module := ModuleCacheTarget
		if cmd.Flags().Changed("derived-data") {
			module.Path = filepath.Join(derived.Path, "ModuleCache.noindex")
		}
		findings, err := runDoctorChecks(cmd.Context(), []internal.Target{derived, SPMTarget, ArchivesTarget, DeviceSupportTarget, module})
		if cmd.Context().Err() != nil {
			return cmd.Context().Err()
		}
		var lines []string
		warnings, failures := 0, 0
		for _, finding := range findings {
			if finding.State == "WARN" {
				warnings++
			}
			if finding.State == "ERROR" {
				failures++
			}
			lines = append(lines, fmt.Sprintf("[%s] %s: %s", finding.State, finding.Name, finding.Detail))
			if finding.Advice != "" {
				lines = append(lines, "  Next: "+finding.Advice)
			}
		}
		lines = append(lines, "", fmt.Sprintf("Doctor: %d errors, %d warnings", failures, warnings))
		if isInteractiveTerminal(cmd.InOrStdin(), cmd.OutOrStdout()) {
			fmt.Fprintln(cmd.OutOrStdout(), tui.RenderSummaryCardFor(cmd.OutOrStdout(), "iOS Development Doctor", lines...))
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines, "\n"))
		}
		return err
	}}
}
