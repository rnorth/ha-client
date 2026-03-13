package cmd

import (
	"runtime"

	"github.com/rnorth/ha-client/internal/output"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		type versionInfo struct {
			Version string `json:"version" yaml:"version"`
			Go      string `json:"go" yaml:"go"`
			OS      string `json:"os" yaml:"os"`
			Arch    string `json:"arch" yaml:"arch"`
		}
		info := versionInfo{
			Version: rootCmd.Version,
			Go:      runtime.Version(),
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		}
		return output.Render(cmd.OutOrStdout(), resolveFormat(), info, nil, renderOpts()...)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
