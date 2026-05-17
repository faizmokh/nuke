package cmd

import "github.com/spf13/cobra"

var archivesCmd = &cobra.Command{
	Use:   "archives",
	Short: "Clean Xcode Archives",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTarget(ArchivesTarget)
	},
}

func init() {
	bindFlags(archivesCmd)
	rootCmd.AddCommand(archivesCmd)
}
