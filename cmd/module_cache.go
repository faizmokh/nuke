package cmd

import "github.com/spf13/cobra"

var moduleCacheCmd = &cobra.Command{
	Use:   "module-cache",
	Short: "Clean the Xcode module cache",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTarget(ModuleCacheTarget)
	},
}

func init() {
	bindFlags(moduleCacheCmd)
	rootCmd.AddCommand(moduleCacheCmd)
}
