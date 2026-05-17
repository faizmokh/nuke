package cmd

import "github.com/spf13/cobra"

var deviceSupportCmd = &cobra.Command{
	Use:   "device-support",
	Short: "Clean iOS DeviceSupport files",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTarget(DeviceSupportTarget)
	},
}

func init() {
	bindFlags(deviceSupportCmd)
	rootCmd.AddCommand(deviceSupportCmd)
}
