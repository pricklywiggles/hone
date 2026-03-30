package cmd

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/store"
)

func init() {
	rootCmd.AddCommand(playlistCmd)
	playlistCmd.AddCommand(playlistCreateCmd)
	playlistCmd.AddCommand(playlistSelectCmd)
	playlistCmd.AddCommand(playlistListCmd)
}

var playlistCmd = &cobra.Command{
	Use:   "playlist",
	Short: "Manage playlists",
}

var playlistCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new playlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		_, err := store.CreatePlaylist(appDB, name)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				fmt.Printf("Playlist %q already exists\n", name)
				return nil
			}
			return err
		}
		fmt.Printf("Created playlist %q\n", name)
		return nil
	},
}

var playlistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all playlists",
	RunE: func(cmd *cobra.Command, args []string) error {
		playlists, err := store.ListPlaylists(appDB)
		if err != nil {
			return err
		}
		if len(playlists) == 0 {
			fmt.Println("No playlists yet.")
			return nil
		}

		activeID := config.ActivePlaylistID()
		fmt.Fprintf(cmd.OutOrStdout(), "  %-20s %s\n", "NAME", "PROBLEMS")
		for _, p := range playlists {
			marker := "  "
			if activeID != nil && *activeID == p.ID {
				marker = "* "
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s%-20s %d\n", marker, p.Name, p.ProblemCount)
		}
		return nil
	},
}

var playlistSelectCmd = &cobra.Command{
	Use:   "select [name]",
	Short: "Set the active playlist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		playlist, err := store.GetPlaylistByName(appDB, name)
		if err != nil {
			if err == sql.ErrNoRows {
				fmt.Printf("No playlist named %q\n", name)
				return nil
			}
			return err
		}
		if err := config.SetActivePlaylist(playlist.ID); err != nil {
			return err
		}
		fmt.Printf("Active playlist set to %q\n", name)
		return nil
	},
}
