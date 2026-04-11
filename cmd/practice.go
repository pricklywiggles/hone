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
		filter := store.PracticeFilter{
			PlaylistID: config.ActivePlaylistID(),
			TopicID:    config.ActiveTopicID(),
		}
		queue, err := store.ListPickQueue(appDB, filter)
		if err != nil {
			return err
		}
		if len(queue) == 0 {
			fmt.Println("No problems yet. Add some with: hone add <url>")
			return nil
		}

		filterName := store.ResolveFilterName(appDB, filter)
		m := tui.NewPracticeModel(appDB, config.BrowserProfileDir(), queue, filter, filterName)
		router := tui.NewRouter(m)
		_, err = tui.Run(router)
		return err
	},
}
