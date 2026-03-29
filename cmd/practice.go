package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(practiceCmd)
}

var practiceCmd = &cobra.Command{
	Use:   "practice",
	Short: "Pick and launch the next problem",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Not implemented yet.")
		return nil
	},
}
