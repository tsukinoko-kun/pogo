package cmd

import (
	"errors"
	"github.com/tsukinoko-kun/pogo/client"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new change",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.Open(".pogo")
		if err != nil {
			return errors.Join(errors.New("open repository"), err)
		}

		if err := c.Push(); err != nil {
			return errors.Join(errors.New("push"), err)
		}

		var parents []string

		if len(args) > 0 {
			parents = args
		} else {
			head, err := c.Head()
			if err != nil {
				return errors.Join(errors.New("get head"), err)
			}
			parents = []string{head}
		}

		if newChangeResp, err := c.NewChange(
			parents,
			nil,
			nil,
		); err != nil {
			return errors.Join(errors.New("create new change"), err)
		} else {
			if err := c.Edit(newChangeResp.GetChangeId()); err != nil {
				return errors.Join(errors.New("edit new change"), err)
			}
		}

		if err := c.Log(); err != nil {
			return errors.Join(errors.New("log"), err)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(newCmd)
}
