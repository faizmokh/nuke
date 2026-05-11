package cmd

import "github.com/spf13/cobra"

var derivedCmd = &cobra.Command{
	Use:   "derived",
	Short: "Clean Xcode DerivedData",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTarget(DerivedTarget)
	},
}

func init() {
	bindFlags(derivedCmd)
	rootCmd.AddCommand(derivedCmd)
}
