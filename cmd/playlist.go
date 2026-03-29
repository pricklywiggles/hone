package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not implemented yet.")
		return nil
	},
}

var playlistSelectCmd = &cobra.Command{
	Use:   "select [name]",
	Short: "Set the active playlist",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not implemented yet.")
		return nil
	},
}

var playlistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all playlists",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not implemented yet.")
		return nil
	},
}
