package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/importer"
	"github.com/pricklywiggles/hone/internal/tui"
)

func init() {
	rootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import problems from a playlist file",
	Long: `Import problems from a file.

URLs are scraped and added. Lines starting with "#" define playlist groups:
all URLs below a "#Name" header are added to (or created in) that playlist.

Example file:

  # Favorites
  https://neetcode.io/problems/two-sum/question
  https://neetcode.io/problems/valid-anagram/question

  # Week 1
  https://neetcode.io/problems/climbing-stairs/question`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		groups, err := importer.ParseImportFile(args[0])
		if err != nil {
			return err
		}

		total := 0
		for _, g := range groups {
			total += len(g.URLs)
		}
		if total == 0 {
			fmt.Println("No URLs found in import file.")
			return nil
		}

		m := tui.NewImportModel(appDB, config.BrowserProfileDir(), groups)
		_, err = tui.RunInline(m)
		return err
	},
}
