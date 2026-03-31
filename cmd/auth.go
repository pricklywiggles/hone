package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
		loginURLs := map[string]string{
			"leetcode": "https://leetcode.com/accounts/login/",
		}
		loginURL := loginURLs[platform]
		if loginURL == "" {
			parts := strings.SplitN(tmpl, "/problems/", 2)
			loginURL = parts[0] + "/"
		}

		if out, _ := exec.Command("pgrep", "-x", "Google Chrome").Output(); len(out) > 0 {
			fmt.Println("Please close Google Chrome before running auth, then try again.")
			return nil
		}

		profileDir := config.BrowserProfileDir()

		chromePath := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		if _, err := os.Stat(chromePath); err != nil {
			return fmt.Errorf("Google Chrome not found at %s", chromePath)
		}

		chromeCmd := exec.Command(chromePath,
			"--user-data-dir="+profileDir,
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-background-networking",
			loginURL,
		)
		if err := chromeCmd.Start(); err != nil {
			return fmt.Errorf("could not launch Chrome: %w", err)
		}

		fmt.Printf("Browser opened at %s.\nLog in, then press Enter here to close the browser...\n", platform)
		bufio.NewReader(os.Stdin).ReadString('\n')
		chromeCmd.Process.Kill()
		chromeCmd.Wait()

		for _, f := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
			os.Remove(filepath.Join(profileDir, f))
		}

		fmt.Printf("Session saved. You can now use 'hone add' with %s URLs.\n", platform)
		return nil
	},
}
