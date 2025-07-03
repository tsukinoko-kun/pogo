//go:build windows
// +build windows

package sysid

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// getMachineID reads the MachineGuid from the registry.
func getMachineID() (string, error) {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return "", err
	}
	defer key.Close()

	s, _, err := key.GetStringValue("MachineGuid")
	if err != nil {
		return "", err
	}
	if s != "" {
		return s, nil
	}
	// fallback to hostname
	s, err = os.Hostname()
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", fmt.Errorf("machine id not found")
	}
	return s, nil
}
