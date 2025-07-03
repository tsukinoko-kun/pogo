//go:build windows
// +build windows

package auth

import (
	"errors"
	"net"
	"os"
	"strings"

	"github.com/Microsoft/go-winio"
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

	var conn net.Conn
	var err error

	// Windows SSH agent uses named pipes
	if strings.HasPrefix(sshAuthSock, `\\.\pipe\`) {
		conn, err = winio.DialPipe(sshAuthSock, nil)
	} else {
		// Fallback for other connection types
		conn, err = net.Dial("tcp", sshAuthSock)
	}

	if err != nil {
		return nil, err
	}

	openConnections = append(openConnections, conn)

	a = agent.NewClient(conn)
	return a, nil
}
