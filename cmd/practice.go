package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/store"
	"github.com/pricklywiggles/hone/internal/tui"
)

func init() {
	rootCmd.AddCommand(practiceCmd)
}

var practiceCmd = &cobra.Command{
	Use:   "practice",
	Short: "Pick and launch the next problem",
	RunE: func(cmd *cobra.Command, args []string) error {
		problem, srsState, isDue, err := store.PickNext(appDB, config.ActivePlaylistID())
		if err != nil {
			return err
		}
		if problem == nil {
			fmt.Println("No problems yet. Add some with: hone add <url>")
			return nil
		}

		m := tui.NewPracticeModel(appDB, config.BrowserProfileDir(), problem, srsState, isDue, config.ActivePlaylistID())
		router := tui.NewRouter(m)
		_, err = tui.Run(router)
		return err
	},
}
