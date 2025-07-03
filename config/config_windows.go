package config

import (
	"os"
	"path/filepath"
)

func getConfigLocation() string {
	return filepath.Join(os.Getenv("APPDATA"), "pogo")
}
