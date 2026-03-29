package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(addCmd)
}

var addCmd = &cobra.Command{
	Use:   "add [url]",
	Short: "Add a problem from a URL",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not implemented yet.")
		return nil
	},
}
