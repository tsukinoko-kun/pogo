package cmd

import (
	"errors"
	"github.com/tsukinoko-kun/pogo/client"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Log the change graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.Open(".pogo")
		if err != nil {
			return errors.Join(errors.New("open repository"), err)
		}

		if err := c.Push(); err != nil {
			return errors.Join(errors.New("push"), err)
		}

		limit, err := cmd.Flags().GetInt32("limit")
		if err != nil {
			limit = 10
		}

		if err := c.LogLimit(limit); err != nil {
			return errors.Join(errors.New("log"), err)
		}

		return nil
	},
}

func init() {
	logCmd.Flags().Int64("limit", 10, "Limit the number of commits to show")
	RootCmd.AddCommand(logCmd)
}
