package cmd

import (
	"encoding/base64"
	"fmt"
	"github.com/tsukinoko-kun/pogo/config"
	"github.com/tsukinoko-kun/pogo/sysid"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Print your author information",
	RunE: func(cmd *cobra.Command, args []string) error {
		if un := config.GetUsername(); len(un) == 0 {
			_, _ = fmt.Fprintf(cmd.OutOrStderr(), "no username set\n")
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "username: %s\n", config.GetUsername())
		}
		if pk, ok := config.GetPublicKey(); !ok {
			_, _ = fmt.Fprintf(cmd.OutOrStderr(), "no public key set\n")
		} else {
			keyType := pk.Type()
			wireFormatBytes := pk.Marshal()
			wireFormat := base64.StdEncoding.EncodeToString(wireFormatBytes)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "public key: %s %s\n", keyType, wireFormat)
		}
		if machine, err := sysid.GetMachineID(); err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStderr(), "failed to get machine ID: %v\n", err)
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "machine id: %s\n", machine)
		}
		return nil
	},
}

func init() {
	RootCmd.AddCommand(whoamiCmd)
}
