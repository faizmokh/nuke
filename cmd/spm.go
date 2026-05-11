package cmd

import "github.com/spf13/cobra"

var spmCmd = &cobra.Command{
	Use:   "spm",
	Short: "Clean Swift Package Manager caches",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTarget(SPMTarget)
	},
}

func init() {
	bindFlags(spmCmd)
	rootCmd.AddCommand(spmCmd)
}
