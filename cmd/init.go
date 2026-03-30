package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pricklywiggles/hone/internal/backup"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/db"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init <backup-file>",
	Short: "Restore from a JSON backup",
	Long: `Restore the hone database from a JSON backup created by "hone export --backup".

This command refuses to run if a database already exists at the default path.
To start fresh from a backup, remove the existing database first:

  rm ~/.local/share/hone/data.db
  hone init backup.json`,
	Args: cobra.ExactArgs(1),
	// Override the parent PersistentPreRunE so we don't open the DB before
	// checking whether it already exists.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return config.Init()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		dbPath := filepath.Join(config.DataDir(), "data.db")
		if _, err := os.Stat(dbPath); err == nil {
			return fmt.Errorf("database already exists at %s\nremove it first if you want to restore from backup", dbPath)
		}

		raw, err := os.ReadFile(args[0])
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
			// Clean up the partially-created DB so the user can retry.
			newDB.Close()
			os.Remove(dbPath)
			return fmt.Errorf("restore: %w", err)
		}

		fmt.Printf("restored %d problem(s), %d playlist(s), %d attempt(s)\n",
			len(data.Problems), len(data.Playlists), len(data.Attempts))
		return nil
	},
}
