package auth

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/tsukinoko-kun/pogo/config"
	"golang.org/x/crypto/ssh"
)

func Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	agent, err := getAgent()
	if err != nil {
		return nil, err
	}
	return agent.Sign(key, data)
}

func CloseOpenConnections() {
	for _, conn := range openConnections {
		_ = conn.Close()
	}
}

func GetPublicKey() (ssh.PublicKey, error) {
	configKey, ok := config.GetPublicKey()
	if !ok {
		return nil, errors.New("no public key set")
	}

	agent, err := getAgent()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to get agent"), err)
	}

	keys, err := agent.List()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to list keys"), err)
	}
	for _, key := range keys {
		if bytes.Equal(key.Marshal(), configKey.Marshal()) {
			return key, nil
		}
	}
	return nil, errors.New("no matching public key found")
}
