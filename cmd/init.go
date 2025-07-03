package cmd

import (
	"errors"
	"fmt"
	"github.com/tsukinoko-kun/pogo/client"
	"github.com/tsukinoko-kun/pogo/config"
	"github.com/tsukinoko-kun/pogo/sysid"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flag("name").Changed {
			return fmt.Errorf("name must be specified")
		}
		name := cmd.Flag("name").Value.String()

		if !cmd.Flag("host").Changed {
			return fmt.Errorf("host must be specified")
		}
		host := cmd.Flag("host").Value.String()

		machine, err := sysid.GetMachineID()
		if err != nil {
			return errors.Join(errors.New("get machine id"), err)
		}

		c, err := client.Init(host, name, config.GetUsername(), machine)
		if err != nil {
			return errors.Join(errors.New("init"), err)
		}
		if err = c.Push(); err != nil {
			return errors.Join(errors.New("push"), err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "repository initialized")

		if err := c.Log(); err != nil {
			return errors.Join(errors.New("log"), err)
		}

		return nil
	},
}

func init() {
	initCmd.Flags().String("name", "", "Name of the repository to initialize")
	initCmd.Flags().String("host", "", "Remote host where the repository should be initialized")
	RootCmd.AddCommand(initCmd)
}
