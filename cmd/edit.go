package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tsukinoko-kun/pogo/client"
)

var editCmd = &cobra.Command{
	Use:     "edit",
	Aliases: []string{"e", "checkout", "switch"},
	Short:   "Edit a different change",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("one change name required")
		}

		c, err := client.Open(".pogo")
		if err != nil {
			return errors.Join(errors.New("open repository"), err)
		}

		if err := c.Push(); err != nil {
			return errors.Join(errors.New("push"), err)
		}

		if err := c.EditName(args[0]); err != nil {
			return errors.Join(errors.New("edit name"), err)
		}

		if err := c.Log(); err != nil {
			return errors.Join(errors.New("log"), err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(editCmd)
}
