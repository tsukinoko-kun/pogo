//go:build darwin
// +build darwin

package sysid

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
)

// getMachineID runs `ioreg` and parses IOPlatformUUID.
func getMachineID() (string, error) {
	out, err := exec.Command(
		"ioreg", "-rd1", "-c", "IOPlatformExpertDevice",
	).Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`"IOPlatformUUID" = "([^"]+)"`)
	m := re.FindSubmatch(out)
	if len(m) < 2 {
		// fallback to hostname
		s, err := os.Hostname()
		if err != nil {
			return "", err
		}
		if s == "" {
			return "", fmt.Errorf("IOPlatformUUID not found")
		}
		return s, nil
	}
	return string(m[1]), nil
}
