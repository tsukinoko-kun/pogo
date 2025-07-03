//go:build !windows
// +build !windows

package auth

import (
	"errors"
	"net"
	"os"

	"golang.org/x/crypto/ssh/agent"
)

var a agent.ExtendedAgent

var openConnections = []net.Conn{}

func getAgent() (agent.ExtendedAgent, error) {
	if a != nil {
		return a, nil
	}

	sshAuthSock, ok := os.LookupEnv("SSH_AUTH_SOCK")
	if !ok {
		return nil, errors.New("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, err
	}

	openConnections = append(openConnections, conn)

	a = agent.NewClient(conn)
	return a, nil
}
