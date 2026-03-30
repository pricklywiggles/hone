package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(authCmd)
}

var authCmd = &cobra.Command{
	Use:   "auth [platform]",
	Short: "Log in to a platform (saves session for scraping)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		platform := strings.ToLower(args[0])
		tmpl := viper.GetString("platforms." + platform + ".url_template")
		if tmpl == "" {
			return fmt.Errorf("unsupported platform %q", platform)
		}
		// Navigate to root of the platform, not a specific problem.
		parts := strings.SplitN(tmpl, "/problems/", 2)
		loginURL := parts[0] + "/"

		profileDir := config.BrowserProfileDir()
		u := launcher.New().UserDataDir(profileDir).Headless(false).MustLaunch()
		browser := rod.New().ControlURL(u).MustConnect()
		browser.MustPage(loginURL)

		fmt.Printf("Browser opened at %s.\nLog in, then press Enter here to save your session...\n", platform)
		bufio.NewReader(os.Stdin).ReadString('\n')
		browser.MustClose()

		fmt.Printf("Session saved. You can now use 'hone add' with %s URLs.\n", platform)
		return nil
	},
}
