package cmd

import (
	"fmt"
	"github.com/tsukinoko-kun/pogo/metadata"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Short:   "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(metadata.Version)
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
