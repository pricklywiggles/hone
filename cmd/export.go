package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pricklywiggles/hone/internal/backup"
	"github.com/pricklywiggles/hone/internal/tui"
	"github.com/spf13/cobra"
)

var (
	exportBackupFlag  bool
	exportPlaylistName string
	exportOutputFile  string
)

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.Flags().BoolVar(&exportBackupFlag, "backup", false, "full JSON backup (SRS state, attempts, playlists)")
	exportCmd.Flags().StringVar(&exportPlaylistName, "playlist", "", "export playlist(s) in text format; omit value for all")
	exportCmd.Flags().Lookup("playlist").NoOptDefVal = "*"
	exportCmd.Flags().StringVarP(&exportOutputFile, "output", "o", "", "write to file instead of stdout")
	exportCmd.MarkFlagsMutuallyExclusive("backup", "playlist")
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export problems or backups",
	Long: `Export problems or create backups.

Without flags, launches an interactive wizard.

  hone export --backup [-o FILE]         Full JSON backup
  hone export --playlist [-o FILE]       All playlists in text format
  hone export --playlist NAME [-o FILE]  Single playlist by name`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if exportBackupFlag {
			return runExportBackup()
		}
		if cmd.Flags().Changed("playlist") {
			return runExportPlaylist()
		}
		// No flags: guided wizard
		m := tui.NewExportWizardModel(appDB)
		_, err := tui.Run(m)
		return err
	},
}

func runExportBackup() error {
	data, err := backup.ExportFullBackup(appDB)
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return writeOutput(string(b) + "\n")
}

func runExportPlaylist() error {
	var content string
	var err error

	if exportPlaylistName == "*" {
		content, err = backup.ExportPlaylistFormat(appDB)
	} else {
		content, err = backup.ExportSinglePlaylistFormat(appDB, exportPlaylistName)
	}
	if err != nil {
		return fmt.Errorf("export: %w", err)
	}
	return writeOutput(content)
}

func writeOutput(content string) error {
	if exportOutputFile != "" {
		if err := os.WriteFile(exportOutputFile, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", exportOutputFile)
		return nil
	}
	fmt.Print(content)
	return nil
}
