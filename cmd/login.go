package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/rnorth/ha-client/internal/client"
	"github.com/rnorth/ha-client/internal/config"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store Home Assistant credentials",
	Long:  "Prompts for server URL and long-lived access token and stores them securely.",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		fmt.Print("Home Assistant server URL (e.g. http://homeassistant.local:8123): ")
		server, _ := reader.ReadString('\n')
		server = strings.TrimSpace(server)

		fmt.Print("Long-lived access token: ")
		tokenBytes, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			// Fallback for non-TTY (e.g. piped input in tests)
			tokenBytes, _ = reader.ReadBytes('\n')
		}
		token := strings.TrimSpace(string(tokenBytes))

		// Verify credentials work
		c, err := client.NewRESTClient(server, token)
		if err != nil {
			return fmt.Errorf("invalid server URL: %w", err)
		}
		if _, err := c.GetInfo(); err != nil {
			return fmt.Errorf("could not connect to Home Assistant: %w", err)
		}

		if err := config.SaveToKeychain(server, token); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
		fmt.Println("Credentials saved successfully.")
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored Home Assistant credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.DeleteFromKeychain(); err != nil {
			return fmt.Errorf("failed to remove credentials: %w", err)
		}
		fmt.Println("Credentials removed.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
}
