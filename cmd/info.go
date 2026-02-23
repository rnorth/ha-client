package cmd

import (
	"os"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show Home Assistant server information",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := resolveConfig()
		if err != nil {
			return err
		}
		c, err := client.NewRESTClient(cfg.Server, cfg.Token)
		if err != nil {
			return err
		}
		info, err := c.GetInfo()
		if err != nil {
			return err
		}
		return output.Render(os.Stdout, resolveFormat(), info, nil)
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
