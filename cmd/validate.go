package cmd

import (
	"github.com/spf13/cobra"
)

// validateCmd represents the validate command
var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate instance stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := initStack(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
