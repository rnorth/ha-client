package cmd

import (
	"fmt"
	"os"

	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var deviceCmd = &cobra.Command{Use: "device", Short: "Manage Home Assistant devices"}

var deviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices",
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		devices, err := wsc.ListDevices()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), devices, []string{"ID", "Name", "Manufacturer", "Model", "AreaID"})
	},
}

var deviceGetCmd = &cobra.Command{
	Use:   "get <device_id>",
	Short: "Get a device by ID or name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		devices, err := wsc.ListDevices()
		if err != nil {
			return err
		}
		for _, d := range devices {
			if d.ID == args[0] || d.Name == args[0] {
				return output.Render(os.Stdout, resolveFormat(), d, nil)
			}
		}
		return fmt.Errorf("device %q not found", args[0])
	},
}

var deviceDescribeCmd = &cobra.Command{
	Use:   "describe <device_id>",
	Short: "Show full device details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		devices, err := wsc.ListDevices()
		if err != nil {
			return err
		}
		for _, d := range devices {
			if d.ID == args[0] || d.Name == args[0] {
				format := resolveFormat()
				if format == output.FormatTable {
					format = output.FormatYAML
				}
				return output.Render(os.Stdout, format, d, nil)
			}
		}
		return fmt.Errorf("device %q not found", args[0])
	},
}

func init() {
	deviceCmd.AddCommand(deviceListCmd, deviceGetCmd, deviceDescribeCmd)
	rootCmd.AddCommand(deviceCmd)
}
