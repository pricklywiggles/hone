package cmd

import (
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/tui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a problem from a URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := ""
		if len(args) > 0 {
			url = args[0]
		}
		m := tui.NewAddModel(appDB, config.BrowserProfileDir(), url)
		_, err := tui.Run(m)
		return err
	},
}
