package cmd

import (
	"errors"

	"github.com/spf13/cobra"
	"github.com/tsukinoko-kun/pogo/client"
	"github.com/tsukinoko-kun/pogo/editor"
)

var describeCmd = &cobra.Command{
	Use:     "describe",
	Aliases: []string{"desc", "reword"},
	Short:   "Describe a change",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.Open(".pogo")
		if err != nil {
			return errors.Join(errors.New("open repository"), err)
		}

		if err := c.Push(); err != nil {
			return errors.Join(errors.New("push"), err)
		}

		var changeName string
		switch len(args) {
		case 0:
			changeName, err = c.Head()
			if err != nil {
				return errors.Join(errors.New("get head"), err)
			}
		case 1:
			changeName = args[0]
		default:
			return errors.New("one change name required")
		}

		m, _ := cmd.Flags().GetString("message")
		if m == "" {
			change, err := c.FindChange(changeName, true)
			if err != nil {
				return errors.Join(errors.New("find change"), err)
			}
			if change != nil && change.Description != nil {
				m = *change.Description
			}

			if newM, err := editor.String("Describe change", m); err != nil {
				return errors.Join(errors.New("edit description"), err)
			} else if newM == m {
				cmd.Println("description unchanged")
				return nil
			} else {
				m = newM
			}
		}

		if m == "" {
			cmd.Println("no description given")
			return nil
		}

		if err := c.Describe(changeName, m); err != nil {
			return errors.Join(errors.New("describe change"), err)
		}

		if err := c.Log(); err != nil {
			return errors.Join(errors.New("log"), err)
		}

		return nil
	},
}

func init() {
	describeCmd.Flags().StringP("message", "m", "", "Use the given message as the change description")
	RootCmd.AddCommand(describeCmd)
}
