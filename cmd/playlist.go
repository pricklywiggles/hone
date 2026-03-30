package cmd

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/store"
	"github.com/pricklywiggles/hone/internal/tui"
)

var (
	plOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	plErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	plDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	plNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Bold(true)
	plHeadStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
	plMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Bold(true)
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
				fmt.Printf("%s Playlist %s already exists\n", plErrStyle.Render("✗"), plNameStyle.Render(name))
				return nil
			}
			return err
		}
		fmt.Printf("%s Created playlist %s\n", plOKStyle.Render("✓"), plNameStyle.Render(name))
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
			fmt.Println(plDimStyle.Render("No playlists yet."))
			return nil
		}

		activeID := config.ActivePlaylistID()
		fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", plHeadStyle.Render(fmt.Sprintf("%-22s %s", "NAME", "PROBLEMS")))
		for _, p := range playlists {
			if activeID != nil && *activeID == p.ID {
				marker := plMarkStyle.Render("* ")
				name := plMarkStyle.Render(fmt.Sprintf("%-22s", p.Name))
				count := plDimStyle.Render(fmt.Sprintf("%d", p.ProblemCount))
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s\n", marker, name, count)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-22s%s\n",
					p.Name,
					plDimStyle.Render(fmt.Sprintf("%d", p.ProblemCount)),
				)
			}
		}
		return nil
	},
}

var playlistSelectCmd = &cobra.Command{
	Use:   "select [name]",
	Short: "Set the active playlist (interactive if no name given)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Direct selection by name.
		if len(args) == 1 {
			name := args[0]
			playlist, err := store.GetPlaylistByName(appDB, name)
			if err != nil {
				if err == sql.ErrNoRows {
					fmt.Printf("%s No playlist named %s\n", plErrStyle.Render("✗"), plNameStyle.Render(name))
					return nil
				}
				return err
			}
			if err := config.SetActivePlaylist(playlist.ID); err != nil {
				return err
			}
			fmt.Printf("%s Active playlist set to %s\n", plOKStyle.Render("✓"), plNameStyle.Render(name))
			return nil
		}

		// Interactive TUI picker.
		playlists, err := store.ListPlaylists(appDB)
		if err != nil {
			return err
		}
		if len(playlists) == 0 {
			fmt.Println(plDimStyle.Render("No playlists yet. Create one with: hone playlist create <name>"))
			return nil
		}

		m := tui.NewPlaylistSelectModel(playlists, config.ActivePlaylistID())
		finalModel, err := tui.Run(m)
		if err != nil {
			return err
		}

		result, ok := finalModel.(tui.PlaylistSelectModel)
		if !ok || result.Canceled() || result.Selected() == nil {
			return nil
		}
		p := result.Selected()
		if err := config.SetActivePlaylist(p.ID); err != nil {
			return err
		}
		fmt.Printf("%s Active playlist set to %s\n", plOKStyle.Render("✓"), plNameStyle.Render(p.Name))
		return nil
	},
}
