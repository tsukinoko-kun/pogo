//go:build linux
// +build linux

package sysid

import (
	"os"
	"strings"
)

// getMachineID returns the contents of /etc/machine-id or /var/lib/dbus/machine-id.
func getMachineID() (string, error) {
	paths := []string{"/etc/machine-id", "/var/lib/dbus/machine-id"}
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			s := strings.TrimSpace(string(b))
			if s != "" {
				return s, nil
			}
		}
	}
	// fall back to hostname
	h, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return h, nil
}
