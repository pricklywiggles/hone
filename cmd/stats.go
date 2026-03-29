package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statsCmd)
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show the statistics dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stats dashboard coming soon.")
		return nil
	},
}
