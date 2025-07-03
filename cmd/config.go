package cmd

import (
	"github.com/tsukinoko-kun/pogo/config"
	"github.com/tsukinoko-kun/pogo/editor"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Edit the user configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		return editor.File(config.GetConfigFileName())
	},
}

func init() {
	RootCmd.AddCommand(configCmd)
}
