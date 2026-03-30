package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/tui"
)

var addFile string

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringVarP(&addFile, "file", "f", "", "file of URLs to add, one per line")
}

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a problem from a URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		if addFile != "" {
			urls, err := readURLFile(addFile)
			if err != nil {
				return err
			}
			m := tui.NewBatchAddModel(appDB, config.BrowserProfileDir(), urls)
			_, err = tui.Run(m)
			return err
		}

		url := ""
		if len(args) > 0 {
			url = args[0]
		}
		m := tui.NewAddModel(appDB, config.BrowserProfileDir(), url)
		_, err := tui.Run(m)
		return err
	},
}

func readURLFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var urls []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}
	return urls, nil
}
