package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(topicsCmd)
}

var topicsCmd = &cobra.Command{
	Use:   "topics",
	Short: "List and filter by topic",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not implemented yet.")
		return nil
	},
}
