package cmd

import (
	"fmt"
	"strings"

	"github.com/pricklywiggles/hone/internal/platform"
	"github.com/pricklywiggles/hone/internal/scraper"
	"github.com/pricklywiggles/hone/internal/store"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a problem from a URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Usage()
		}

		plat, slug, err := platform.ParseURL(args[0])
		if err != nil {
			return fmt.Errorf("parsing URL: %w", err)
		}

		meta, err := scraper.Scrape(plat, slug)
		if err != nil {
			return fmt.Errorf("scraping problem: %w", err)
		}

		_, err = store.InsertProblem(appDB, plat, slug, meta.Title, meta.Difficulty, meta.Topics)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Println("Problem already exists")
				return nil
			}
			return fmt.Errorf("inserting problem: %w", err)
		}

		fmt.Printf("Added: %s (%s) [%s]\n", meta.Title, meta.Difficulty, strings.Join(meta.Topics, ", "))
		return nil
	},
}
