package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/pricklywiggles/hone/internal/backup"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/db"
	"github.com/pricklywiggles/hone/internal/importer"
	"github.com/pricklywiggles/hone/internal/tui"
)

var (
	importPlaylistFile string
	importBackupFile   string
	importURL          string
)

func init() {
	rootCmd.AddCommand(importCmd)
	importCmd.Flags().StringVar(&importPlaylistFile, "playlist", "", "import from playlist file")
	importCmd.Flags().StringVar(&importBackupFile, "backup", "", "restore from JSON backup")
	importCmd.Flags().StringVar(&importURL, "url", "", "add a single problem URL")
	importCmd.MarkFlagsMutuallyExclusive("playlist", "backup", "url")
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import problems or restore from backup",
	Long: `Import problems or restore from a backup.

Without flags, launches an interactive wizard.

  hone import --playlist FILE    Import from playlist file (# headers define playlists)
  hone import --backup FILE      Restore from JSON backup
  hone import --url URL          Add a single problem by URL`,
	Args: cobra.NoArgs,
	// Override PersistentPreRunE: backup mode skips DB open since DB may not exist.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Init(); err != nil {
			return err
		}
		if importBackupFile != "" {
			return nil
		}
		var err error
		appDB, err = db.Open(config.DataDir())
		return err
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if importBackupFile != "" {
			return runImportBackup()
		}
		if importPlaylistFile != "" {
			return runImportPlaylist()
		}
		if importURL != "" {
			return runImportURL()
		}
		// No flags: guided wizard
		m := tui.NewImportWizardModel(appDB, config.DataDir(), config.BrowserProfileDir())
		_, err := tui.Run(m)
		return err
	},
}

func runImportBackup() error {
	dbPath := filepath.Join(config.DataDir(), "data.db")
	if _, err := os.Stat(dbPath); err == nil {
		return fmt.Errorf("database already exists at %s\nremove it first if you want to restore from backup", dbPath)
	}

	raw, err := os.ReadFile(importBackupFile)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	var data backup.BackupData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("parse backup: %w", err)
	}

	newDB, err := db.Open(config.DataDir())
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	defer newDB.Close()

	if err := backup.RestoreFromBackup(newDB, data); err != nil {
		newDB.Close()
		os.Remove(dbPath)
		return fmt.Errorf("restore: %w", err)
	}

	fmt.Printf("restored %d problem(s), %d playlist(s), %d attempt(s)\n",
		len(data.Problems), len(data.Playlists), len(data.Attempts))
	return nil
}

func runImportPlaylist() error {
	groups, err := importer.ParseImportFile(importPlaylistFile)
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
	final, err := tui.RunInline(m)
	if err != nil {
		return err
	}
	printFailedSummary(final.(tui.ImportModel).FailedURLs())
	return nil
}

func runImportURL() error {
	m := tui.NewAddModel(appDB, config.BrowserProfileDir(), importURL)
	_, err := tui.Run(m)
	return err
}

func printFailedSummary(urls []string) {
	if len(urls) == 0 {
		return
	}
	fmt.Printf("\n%d URL(s) failed. See %s\n", len(urls), config.FailedURLsPath())
}
