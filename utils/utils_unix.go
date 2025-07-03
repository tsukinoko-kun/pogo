//go:build !windows
// +build !windows

package utils

import "os"

func IsExecutable(absPath string) *bool {
	stat, err := os.Stat(absPath)
	if err != nil {
		return Ptr(false)
	}
	return Ptr(stat.Mode().Perm()&0111 != 0)
}

func SetExecutable(absPath string, executable bool) error {
	stat, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	mode := stat.Mode()
	if executable {
		mode |= 0111
	} else {
		mode &= 0666
	}
	return os.Chmod(absPath, mode)
}
