package config

import (
	"os"
	"path/filepath"
)

func getConfigLocation() string {
	if xdgConfigHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok {
		return filepath.Join(xdgConfigHome, "pogo")
	}
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "pogo")
}
