package main

import (
	"github.com/tsukinoko-kun/pogo/auth"
	"github.com/tsukinoko-kun/pogo/cmd"
)

func main() {
	defer auth.CloseOpenConnections()
	cmd.Execute()
}
