package cmd

import "github.com/spf13/cobra"

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Clean DerivedData and SPM caches",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAll()
	},
}

func init() {
	bindFlags(allCmd)
	rootCmd.AddCommand(allCmd)
}
