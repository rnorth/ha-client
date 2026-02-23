package cmd

import (
	"fmt"
	"os"

	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var areaCmd = &cobra.Command{Use: "area", Short: "Manage Home Assistant areas"}

var areaListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all areas",
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		areas, err := wsc.ListAreas()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), areas, nil)
	},
}

var areaGetCmd = &cobra.Command{
	Use:   "get <area_id>",
	Short: "Get a specific area by ID or name",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		areas, err := wsc.ListAreas()
		if err != nil {
			return err
		}
		for _, a := range areas {
			if a.AreaID == args[0] || a.Name == args[0] {
				return output.Render(os.Stdout, resolveFormat(), a, nil)
			}
		}
		return fmt.Errorf("area %q not found", args[0])
	},
}

var areaCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		area, err := wsc.CreateArea(args[0])
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), area, nil)
	},
}

var areaDeleteCmd = &cobra.Command{
	Use:   "delete <area_id>",
	Short: "Delete an area",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		wsc, err := newWSClient()
		if err != nil {
			return err
		}
		defer wsc.Close()
		if err := wsc.DeleteArea(args[0]); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "Area deleted.")
		return nil
	},
}

func init() {
	areaCmd.AddCommand(areaListCmd, areaGetCmd, areaCreateCmd, areaDeleteCmd)
	rootCmd.AddCommand(areaCmd)
}
