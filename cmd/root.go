package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tsukinoko-kun/pogo/client"
)

const localRepoFileName = ".pogo"

var RootCmd = &cobra.Command{
	Use:   "pogo",
	Short: "Version control system",
	Long: `Centralized version control system with a workflow inspired by Jujutsu.

All repository data is stored in a PostgreSQL database.

It is not distributed like Git.

You don't push your changes to the central server, you just make your changes and Pogo will take care of the rest.

You can describe your changes before, during and after you made them.
By starting a new change, you commit your changes and make them immutable.
All changes are directly stored in the repository, so your team can use your changes right away.`,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := client.Open(".pogo")
		if err != nil {
			return
		}

		if err := c.Push(); err != nil {
			cmd.PrintErrf("push: %s\n", err.Error())
			return
		}

		if err := c.Log(); err != nil {
			return
		}
	},
}

func init() {
	RootCmd.SilenceUsage = true
	RootCmd.DisableAutoGenTag = true
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
