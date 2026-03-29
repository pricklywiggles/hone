package cmd

import (
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
	"github.com/pricklywiggles/hone/internal/db"
	"github.com/spf13/cobra"
)

var appDB *sqlx.DB

var rootCmd = &cobra.Command{
	Use:   "hone",
	Short: "Practice coding problems with spaced repetition",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Init(); err != nil {
			return fmt.Errorf("config: %w", err)
		}
		var err error
		appDB, err = db.Open(config.DataDir())
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if appDB != nil {
			return appDB.Close()
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stats dashboard coming soon.")
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
