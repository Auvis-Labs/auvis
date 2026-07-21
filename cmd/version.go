package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of AUVIS",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("auvis version %s\n", version)
	},
}
