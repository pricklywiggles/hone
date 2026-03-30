package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pricklywiggles/hone/internal/backup"
	"github.com/spf13/cobra"
)

var exportBackup bool
var exportOutput string

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().BoolVar(&exportBackup, "backup", false, "export full JSON backup (SRS state, attempts, playlists)")
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "write to file instead of stdout")
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export problems to a file",
	Long: `Export problems to a file.

By default, exports in playlist import format (compatible with "hone import"):
each playlist appears as a "# Name" header followed by its problem URLs.
Problems not in any playlist appear at the top with no header.

With --backup, exports a full JSON snapshot including SRS state, attempt
history, and playlist memberships. Use "hone init" to restore from a backup.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var content string
		var err error

		if exportBackup {
			data, err := backup.ExportFullBackup(appDB)
			if err != nil {
				return fmt.Errorf("export: %w", err)
			}
			b, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return err
			}
			content = string(b) + "\n"
		} else {
			content, err = backup.ExportPlaylistFormat(appDB)
			if err != nil {
				return fmt.Errorf("export: %w", err)
			}
		}

		if exportOutput != "" {
			if err := os.WriteFile(exportOutput, []byte(content), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", exportOutput)
			return nil
		}

		fmt.Print(content)
		return nil
	},
}
