package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tsukinoko-kun/pogo/client"
	"github.com/tsukinoko-kun/pogo/colors"
)

var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "List all conflicts in a change",
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
			return fmt.Errorf("one change name required")
		}

		conflicts, err := c.Conflicts(changeName)
		if err != nil {
			return errors.Join(errors.New("get conflicts"), err)
		}

		if len(conflicts) == 0 {
			fmt.Println(colors.BrightBlack + "(no conflicts)" + colors.Reset)
		} else {
			fmt.Printf("%s(%d conflicts)%s\n", colors.Red, len(conflicts), colors.Reset)
			for _, conflict := range conflicts {
				fmt.Println(conflict)
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(conflictsCmd)
}
